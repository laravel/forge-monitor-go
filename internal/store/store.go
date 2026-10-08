// Package store owns the SQLite database: schema, sample inserts, alert
// inserts, the monitor evaluation queries, and retention pruning. It reuses the
// exact evaluation SQL from the PHP tool so alert decisions are identical.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/laravel/forge-monitor-go/internal/config"

	_ "modernc.org/sqlite"
)

// timeLayout is Laravel's datetime string format, written in UTC. The
// evaluation queries compare against DATETIME('NOW', ...) which is UTC, so
// timestamps must be stored in UTC for the windows to line up.
const timeLayout = "2006-01-02 15:04:05"

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
}

// StateRow is one row of an evaluation query result: the sample's computed
// state and the last recorded alert state for the monitor.
type StateRow struct {
	CurrentState string
	LastState    string
}

// Open opens the SQLite database and ensures the schema exists. If the database
// file does not exist yet it (and its parent directory) are created; an
// existing Forge database is used as-is. It uses the pure-Go modernc driver so
// the binary stays static, forces a single connection to serialize writes, and
// leaves journal_mode untouched so the on-disk file format is unchanged.
func Open(path string, log *slog.Logger) (*Store, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		if dir := filepath.Dir(path); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("creating database directory %s: %w", dir, err)
			}
		}
		if log != nil {
			log.Info("database not found, creating", "path", path)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ensureSchema() error {
	for _, stmt := range schemaStatements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("ensuring schema: %w", err)
		}
	}
	return nil
}

// now returns the current UTC timestamp in Laravel's string format.
func now() string {
	return time.Now().UTC().Format(timeLayout)
}

// InsertLoadAvg stores a CPU load sample. load_avg and load_avg_percent are
// bound as float64 (matching PHP floats under INTEGER affinity); cpus as int64.
func (s *Store) InsertLoadAvg(loadAvg, loadAvgPercent float64, cpus int64) error {
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO load_avgs (load_avg, load_avg_percent, cpus, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		loadAvg, loadAvgPercent, cpus, ts, ts,
	)
	return err
}

// InsertMemory stores a memory sample. total is kB (int64); available, used and
// free are percentages (float64). Note that, matching the PHP quirk, available
// carries the free percentage rather than available kB.
func (s *Store) InsertMemory(totalKB int64, availablePercent, usedPercent, freePercent float64) error {
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO memory_usages (total, available, used, free, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		totalKB, availablePercent, usedPercent, freePercent, ts, ts,
	)
	return err
}

// InsertDisk stores a disk sample. total is bytes (int64); free and used are
// percentages (float64).
func (s *Store) InsertDisk(totalBytes int64, freePercent, usedPercent float64) error {
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO disk_usages (total, free, used, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		totalBytes, freePercent, usedPercent, ts, ts,
	)
	return err
}

// InsertAlert records an alert state transition for a monitor.
func (s *Store) InsertAlert(monitorID, monitorType, state string) error {
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO alerts (monitor_id, monitor_type, monitor_state, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		monitorID, monitorType, state, ts, ts,
	)
	return err
}

// Prune deletes sample rows older than the given number of days, matching
// CleanupCommand. The alerts table is intentionally left untouched.
func (s *Store) Prune(days int) error {
	cutoff := fmt.Sprintf("-%d DAYS", days)
	for _, table := range []string{"load_avgs", "disk_usages", "memory_usages"} {
		stmt := fmt.Sprintf(`DELETE FROM %s WHERE created_at <= DATETIME('NOW', ?)`, table)
		if _, err := s.db.Exec(stmt, cutoff); err != nil {
			return fmt.Errorf("pruning %s: %w", table, err)
		}
	}
	return nil
}

// operator maps the config operator to the SQL comparison, matching
// AbstractStat::getOperator (gte -> >=, anything else -> <=).
func operator(op string) string {
	if op == "gte" {
		return ">="
	}
	return "<="
}

// Evaluate runs the appropriate evaluation query for the monitor and returns
// the resulting rows in created_at DESC order. The two query shapes and their
// parameter order are copied verbatim from the PHP Stat classes.
func (s *Store) Evaluate(m config.Monitor) ([]StateRow, error) {
	op := operator(m.Operator)

	if m.Type == config.TypeDisk {
		query := fmt.Sprintf(`SELECT
    CASE WHEN used %s ? THEN 'ALERT' ELSE 'OK' END AS currentState,
    IFNULL(alerts.monitor_state, 'UNKNOWN') AS lastState
FROM (
    SELECT * FROM disk_usages ORDER BY created_at DESC LIMIT 1
) _samples
LEFT JOIN (SELECT * FROM alerts WHERE monitor_id = ? AND monitor_type = ? ORDER BY created_at DESC LIMIT 1) alerts`, op)

		return s.queryStates(query, m.Threshold, m.Key, m.Type)
	}

	table, col := windowedTarget(m.Type)
	query := fmt.Sprintf(`SELECT
    CASE WHEN %s %s ? THEN 'ALERT' ELSE 'OK' END AS currentState,
    IFNULL(alerts.monitor_state, 'UNKNOWN') AS lastState
FROM (
    SELECT * FROM %s WHERE created_at >= DATETIME('NOW', ?) ORDER BY created_at DESC LIMIT ?
) _samples
LEFT JOIN (SELECT * FROM alerts WHERE monitor_id = ? AND monitor_type = ? ORDER BY created_at DESC LIMIT 1) alerts`, col, op, table)

	window := fmt.Sprintf("-%d minutes", m.Minutes+1)
	limit := m.Minutes + 1

	return s.queryStates(query, m.Threshold, window, limit, m.Key, m.Type)
}

// windowedTarget returns the (table, column) evaluated for a windowed monitor.
func windowedTarget(typ string) (string, string) {
	switch typ {
	case config.TypeCPULoad:
		return "load_avgs", "load_avg_percent"
	case config.TypeFreeMemory:
		return "memory_usages", "free"
	case config.TypeUsedMemory:
		return "memory_usages", "used"
	}
	return "", ""
}

func (s *Store) queryStates(query string, args ...any) ([]StateRow, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating monitor: %w", err)
	}
	defer rows.Close()

	var out []StateRow
	for rows.Next() {
		var r StateRow
		if err := rows.Scan(&r.CurrentState, &r.LastState); err != nil {
			return nil, fmt.Errorf("scanning evaluation row: %w", err)
		}
		out = append(out, r)
	}

	return out, rows.Err()
}
