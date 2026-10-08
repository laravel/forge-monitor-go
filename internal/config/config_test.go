package config

import (
	"os"
	"path/filepath"
	"testing"
)

const exampleMonitor = `[monitor-1]
type = "disk"
operator = "gte"
threshold = 10
token = "foobarbaz"

[monitor-2]
type = "free_memory"
operator = "lte"
threshold = 25
minutes = 5
token = "foobarbaz"
`

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(exampleMonitor), 0o644); err != nil {
		t.Fatal(err)
	}

	monitors, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 2 {
		t.Fatalf("want 2 monitors, got %d", len(monitors))
	}

	byKey := map[string]Monitor{}
	for _, m := range monitors {
		byKey[m.Key] = m
	}

	disk, ok := byKey["monitor-1"]
	if !ok {
		t.Fatal("monitor-1 missing")
	}
	if disk.Type != TypeDisk || disk.Operator != "gte" || disk.Threshold != 10 || disk.Token != "foobarbaz" {
		t.Fatalf("disk monitor parsed wrong: %+v", disk)
	}
	if disk.Minutes != 1 {
		t.Fatalf("disk minutes should be forced to 1, got %d", disk.Minutes)
	}
	if disk.Category() != CategoryDisk {
		t.Fatalf("disk category = %q", disk.Category())
	}

	mem, ok := byKey["monitor-2"]
	if !ok {
		t.Fatal("monitor-2 missing")
	}
	if mem.Type != TypeFreeMemory || mem.Operator != "lte" || mem.Threshold != 25 || mem.Minutes != 5 {
		t.Fatalf("memory monitor parsed wrong: %+v", mem)
	}
	if mem.Category() != CategoryMemory {
		t.Fatalf("memory category = %q", mem.Category())
	}
}

func TestFindOrderHomeFirst(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	// Only the working dir has the file.
	if err := os.WriteFile(filepath.Join(work, FileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Find([]string{home, work}); !ok || got != filepath.Join(work, FileName) {
		t.Fatalf("Find should locate work file, got %q ok=%v", got, ok)
	}

	// Now home also has one: home should win.
	if err := os.WriteFile(filepath.Join(home, FileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Find([]string{home, work}); got != filepath.Join(home, FileName) {
		t.Fatalf("Find should prefer home, got %q", got)
	}
}

func TestLoadMissingFileIsNotError(t *testing.T) {
	monitors, err := Load([]string{t.TempDir()})
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if monitors != nil {
		t.Fatalf("want nil monitors, got %v", monitors)
	}
}
