package store

import (
	"path/filepath"
	"testing"

	"github.com/laravel/forge-monitor-go/internal/config"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "database.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSchemaTablesExist(t *testing.T) {
	s := openTemp(t)
	for _, table := range []string{"alerts", "disk_usages", "memory_usages", "load_avgs"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestEvaluateDiskSingleSample(t *testing.T) {
	s := openTemp(t)

	if err := s.InsertDisk(100_000_000, 10, 90); err != nil { // used 90%
		t.Fatal(err)
	}

	m := config.Monitor{Key: "monitor-1", Type: config.TypeDisk, Operator: "gte", Threshold: 80, Minutes: 1}
	rows, err := s.Evaluate(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].CurrentState != "ALERT" {
		t.Fatalf("used 90 >= 80 should be ALERT, got %s", rows[0].CurrentState)
	}
	if rows[0].LastState != "UNKNOWN" {
		t.Fatalf("no prior alert -> UNKNOWN, got %s", rows[0].LastState)
	}
}

func TestEvaluateMemoryWindowedAndAlertJoin(t *testing.T) {
	s := openTemp(t)

	// free 20% now.
	if err := s.InsertMemory(1000, 20, 80, 20); err != nil {
		t.Fatal(err)
	}

	m := config.Monitor{Key: "monitor-2", Type: config.TypeFreeMemory, Operator: "lte", Threshold: 25, Minutes: 3}
	rows, err := s.Evaluate(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row within window, got %d", len(rows))
	}
	if rows[0].CurrentState != "ALERT" {
		t.Fatalf("free 20 <= 25 should be ALERT, got %s", rows[0].CurrentState)
	}
	if rows[0].LastState != "UNKNOWN" {
		t.Fatalf("no prior alert -> UNKNOWN, got %s", rows[0].LastState)
	}

	// Record an alert; now the join should reflect it.
	if err := s.InsertAlert(m.Key, m.Type, "ALERT"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Evaluate(m)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].LastState != "ALERT" {
		t.Fatalf("after recording alert, lastState should be ALERT, got %s", rows[0].LastState)
	}
}

func TestPruneKeepsAlerts(t *testing.T) {
	s := openTemp(t)
	if err := s.InsertDisk(1, 50, 50); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertAlert("m", "disk", "OK"); err != nil {
		t.Fatal(err)
	}
	// Pruning recent data should delete nothing.
	if err := s.Prune(7); err != nil {
		t.Fatal(err)
	}
	var diskCount, alertCount int
	s.db.QueryRow(`SELECT COUNT(*) FROM disk_usages`).Scan(&diskCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM alerts`).Scan(&alertCount)
	if diskCount != 1 || alertCount != 1 {
		t.Fatalf("recent rows should survive prune: disk=%d alert=%d", diskCount, alertCount)
	}
}
