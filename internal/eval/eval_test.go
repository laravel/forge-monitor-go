package eval

import (
	"reflect"
	"testing"

	"github.com/laravel/forge-monitor-go/internal/store"
)

func row(current, last string) store.StateRow {
	return store.StateRow{CurrentState: current, LastState: last}
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		minutes int
		rows    []store.StateRow
		want    []string
	}{
		{
			name:    "no data",
			minutes: 3,
			rows:    nil,
			want:    nil,
		},
		{
			name:    "not enough data, first seen, currently alert -> initial ALERT",
			minutes: 3,
			rows:    []store.StateRow{row(StateAlert, StateUnknown)},
			want:    []string{StateAlert},
		},
		{
			name:    "not enough data, first seen, currently ok -> initial OK",
			minutes: 3,
			rows:    []store.StateRow{row(StateOK, StateUnknown)},
			want:    []string{StateOK},
		},
		{
			name:    "not enough data, already known -> nothing",
			minutes: 3,
			rows:    []store.StateRow{row(StateAlert, StateOK)},
			want:    nil,
		},
		{
			name:    "enough data, never alerted, all ok -> first-time OK",
			minutes: 3,
			rows:    []store.StateRow{row(StateOK, StateUnknown), row(StateOK, StateUnknown), row(StateOK, StateUnknown)},
			want:    []string{StateOK},
		},
		{
			name:    "enough data, never alerted, all alert -> first-time ALERT",
			minutes: 3,
			rows:    []store.StateRow{row(StateAlert, StateUnknown), row(StateAlert, StateUnknown), row(StateAlert, StateUnknown)},
			want:    []string{StateAlert},
		},
		{
			name:    "alert streak met, previously OK -> ALERT",
			minutes: 3,
			rows:    []store.StateRow{row(StateAlert, StateOK), row(StateAlert, StateOK), row(StateAlert, StateOK)},
			want:    []string{StateAlert},
		},
		{
			name:    "alert streak met, already alerting -> nothing",
			minutes: 3,
			rows:    []store.StateRow{row(StateAlert, StateAlert), row(StateAlert, StateAlert), row(StateAlert, StateAlert)},
			want:    nil,
		},
		{
			name:    "alert streak exceeds minutes -> nothing (fires only at ==minutes)",
			minutes: 3,
			// 4 consecutive alerts: streak 4 != minutes 3, so no new alert.
			rows: []store.StateRow{row(StateAlert, StateOK), row(StateAlert, StateOK), row(StateAlert, StateOK), row(StateAlert, StateOK)},
			want: nil,
		},
		{
			name:    "recovery: currently OK, was alerting -> OK",
			minutes: 3,
			rows:    []store.StateRow{row(StateOK, StateAlert), row(StateOK, StateAlert), row(StateOK, StateAlert)},
			want:    []string{StateOK},
		},
		{
			name:    "steady OK, already OK -> nothing",
			minutes: 3,
			rows:    []store.StateRow{row(StateOK, StateOK), row(StateOK, StateOK), row(StateOK, StateOK)},
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.minutes, tc.rows)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Decide(%d, %v) = %v, want %v", tc.minutes, tc.rows, got, tc.want)
			}
		})
	}
}

// TestDecideAlertFiresExactlyAtStreak verifies the "==minutes" boundary: the
// alert fires only when the consecutive count equals minutes, not before.
func TestDecideAlertFiresExactlyAtStreak(t *testing.T) {
	minutes := 3

	// 2 alert rows: total (2) < minutes (3) -> not enough data branch.
	two := []store.StateRow{row(StateAlert, StateOK), row(StateAlert, StateOK)}
	if got := Decide(minutes, two); got != nil {
		t.Fatalf("with 2 rows want nil, got %v", got)
	}

	// Exactly 3 alert rows, previously OK -> ALERT.
	three := []store.StateRow{row(StateAlert, StateOK), row(StateAlert, StateOK), row(StateAlert, StateOK)}
	if got := Decide(minutes, three); !reflect.DeepEqual(got, []string{StateAlert}) {
		t.Fatalf("with 3 rows want [ALERT], got %v", got)
	}
}
