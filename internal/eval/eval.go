// Package eval ports the alert state machine from app/Stats/AbstractStat.php.
// Decide is a pure function over evaluation rows so it can be tested in
// isolation; Evaluator wires it to the store and notifier, inserting an alert
// row and then posting, in that order (as App\Alert::createForMonitor does).
package eval

import (
	"context"
	"log/slog"

	"github.com/laravel/forge-monitor-go/internal/config"
	"github.com/laravel/forge-monitor-go/internal/notify"
	"github.com/laravel/forge-monitor-go/internal/store"
)

// Alert states, matching AbstractStat's OK/ALERT/UNKNOWN constants.
const (
	StateOK      = "OK"
	StateAlert   = "ALERT"
	StateUnknown = "UNKNOWN"
)

// Decide replays the streak state machine over the rows (in created_at DESC
// order) and returns the alert states that should be created, in order. It is a
// literal port of AbstractStat::testResults + handleState + reportInitialState.
func Decide(minutes int, rows []store.StateRow) []string {
	total := len(rows)

	// Not enough data to check a streak, but report an initial state if the
	// monitor has never alerted yet.
	if total < minutes {
		if total > 0 && rows[0].LastState == StateUnknown {
			return []string{rows[0].CurrentState}
		}
		return nil
	}

	var lastState, lastAlertState string
	alertStreak := 0

	for _, r := range rows {
		switch r.CurrentState {
		case StateOK:
			if lastState == StateAlert {
				alertStreak = 0
			}
		case StateAlert:
			alertStreak++
		}

		lastState = r.CurrentState
		lastAlertState = r.LastState
	}

	// First-time notification: no prior alert recorded.
	if lastAlertState == StateUnknown {
		if lastState == StateOK {
			return []string{StateOK}
		}
		return []string{StateAlert}
	}

	// Notification updates.
	if lastState == StateAlert {
		if alertStreak == minutes && lastAlertState != StateAlert {
			return []string{StateAlert}
		}
	} else if lastState == StateOK {
		if lastAlertState != StateOK {
			return []string{StateOK}
		}
	}

	return nil
}

// Evaluator runs monitor evaluation against the store and notifies on state
// changes.
type Evaluator struct {
	store    *store.Store
	notifier *notify.Notifier
	log      *slog.Logger
}

// NewEvaluator builds an Evaluator.
func NewEvaluator(s *store.Store, n *notify.Notifier, log *slog.Logger) *Evaluator {
	return &Evaluator{store: s, notifier: n, log: log}
}

// Evaluate runs one monitor's evaluation query, applies the state machine, and
// for each resulting state writes an alert row and posts to Forge. A failed
// post is logged but does not undo the alert row, so the next cycle sees the
// updated last-alert state.
func (e *Evaluator) Evaluate(ctx context.Context, m config.Monitor) error {
	rows, err := e.store.Evaluate(m)
	if err != nil {
		return err
	}

	for _, state := range Decide(m.Minutes, rows) {
		if err := e.store.InsertAlert(m.Key, m.Type, state); err != nil {
			return err
		}

		if err := e.notifier.Alert(ctx, m, state); err != nil {
			e.log.Warn("failed to post monitor alert",
				"monitor", m.Key, "type", m.Type, "state", state, "error", err)
		} else {
			e.log.Info("monitor alert sent",
				"monitor", m.Key, "type", m.Type, "state", state)
		}
	}

	return nil
}
