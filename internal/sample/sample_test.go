package sample

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cpuinfo := "processor\t: 0\nvendor_id\t: X\nprocessor\t: 1\nprocessor\t: 2\nprocessor\t: 3\n"
	if err := os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte(cpuinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loadavg"), []byte("2.00 1.50 1.00 1/234 5678\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meminfo := "MemTotal:        1000 kB\nMemFree:          100 kB\nMemAvailable:     250 kB\n"
	if err := os.WriteFile(filepath.Join(dir, "meminfo"), []byte(meminfo), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadSample(t *testing.T) {
	s := &Sampler{ProcRoot: writeProc(t)}

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK {
		t.Fatal("expected OK sample")
	}
	if got.Cpus != 4 {
		t.Fatalf("cpus = %d, want 4", got.Cpus)
	}
	if got.LoadAvg != 2.0 {
		t.Fatalf("loadAvg = %v, want 2.0", got.LoadAvg)
	}
	// 2.0 / 4 * 100 = 50
	if got.LoadAvgPercent != 50 {
		t.Fatalf("loadAvgPercent = %v, want 50", got.LoadAvgPercent)
	}
}

func TestMemorySample(t *testing.T) {
	s := &Sampler{ProcRoot: writeProc(t)}

	got, err := s.Memory()
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK {
		t.Fatal("expected OK sample")
	}
	if got.TotalKB != 1000 {
		t.Fatalf("total = %d, want 1000", got.TotalKB)
	}
	// used = 1000 - 250 = 750 -> usedPct 75, freePct 25.
	if got.UsedPercent != 75 {
		t.Fatalf("usedPercent = %v, want 75", got.UsedPercent)
	}
	if got.FreePercent != 25 {
		t.Fatalf("freePercent = %v, want 25", got.FreePercent)
	}
	// The PHP quirk: available column carries the free percentage.
	if got.AvailablePercent != 25 {
		t.Fatalf("availablePercent = %v, want 25 (free%%)", got.AvailablePercent)
	}
}

func TestLoadSampleUnreadable(t *testing.T) {
	s := &Sampler{ProcRoot: t.TempDir()} // no cpuinfo
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.OK {
		t.Fatal("expected OK=false when /proc/cpuinfo is missing")
	}
}

func TestDiskSampleLive(t *testing.T) {
	// Smoke test against the real filesystem (statfs works on macOS and Linux).
	s := &Sampler{DiskPath: "/"}
	got, err := s.Disk()
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK {
		t.Fatal("expected OK disk sample")
	}
	if got.TotalBytes <= 0 {
		t.Fatalf("total bytes = %d", got.TotalBytes)
	}
	if got.UsedPercent < 0 || got.UsedPercent > 100 {
		t.Fatalf("usedPercent out of range: %v", got.UsedPercent)
	}
	if diff := got.UsedPercent + got.FreePercent; diff < 99 || diff > 101 {
		t.Fatalf("used+free should be ~100, got %v", diff)
	}
}
