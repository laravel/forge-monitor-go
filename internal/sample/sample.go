// Package sample collects system metrics natively from /proc and statfs,
// reproducing the values the PHP samplers stored (app/Stats/LoadAvg.php,
// Memory.php, DiskSpace.php) without shelling out to grep/awk/wc.
package sample

// Sampler reads metrics. ProcRoot and DiskPath are configurable so tests can
// point at fixtures; production uses the defaults.
type Sampler struct {
	// ProcRoot is the directory holding cpuinfo, loadavg and meminfo.
	ProcRoot string
	// DiskPath is the filesystem path measured for disk usage.
	DiskPath string
}

// New returns a Sampler configured for a live Linux host: /proc and /.
func New() *Sampler {
	return &Sampler{ProcRoot: "/proc", DiskPath: "/"}
}

// LoadSample is a CPU load sample. OK is false when the source is unreadable
// (e.g. non-Linux), in which case no row should be written, matching the PHP
// is_readable guard.
type LoadSample struct {
	LoadAvg        float64
	LoadAvgPercent float64
	Cpus           int64
	OK             bool
}

// MemSample is a memory sample. Available carries the free percentage to match
// the PHP quirk. OK is false when /proc/meminfo is unreadable.
type MemSample struct {
	TotalKB          int64
	AvailablePercent float64
	UsedPercent      float64
	FreePercent      float64
	OK               bool
}

// DiskSample is a disk-usage sample for the measured path.
type DiskSample struct {
	TotalBytes  int64
	FreePercent float64
	UsedPercent float64
	OK          bool
}
