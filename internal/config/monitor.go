package config

// Monitor types, mirroring app/Monitors/Monitor.php.
const (
	TypeDisk       = "disk"
	TypeCPULoad    = "cpu_load"
	TypeFreeMemory = "free_memory"
	TypeUsedMemory = "used_memory"
)

// Category groups monitor types by the sampler that feeds them.
const (
	CategoryDisk   = "disk"
	CategoryLoad   = "load"
	CategoryMemory = "memory"
)

// Monitor is a single configured monitor, equivalent to App\Monitors\Monitor.
type Monitor struct {
	Key       string
	Type      string
	Operator  string
	Threshold float64
	Minutes   int
	Token     string
}

// Category returns the sampler category this monitor's type belongs to.
func (m Monitor) Category() string {
	switch m.Type {
	case TypeDisk:
		return CategoryDisk
	case TypeCPULoad:
		return CategoryLoad
	case TypeFreeMemory, TypeUsedMemory:
		return CategoryMemory
	}
	return ""
}

// newMonitor normalizes a parsed section into a Monitor, applying the same
// coercions as the PHP constructor: threshold to float, minutes to int, and
// disk monitors forced to a single check.
func newMonitor(key, typ, operator string, threshold float64, minutes int, token string) Monitor {
	if typ == TypeDisk {
		minutes = 1
	}

	return Monitor{
		Key:       key,
		Type:      typ,
		Operator:  operator,
		Threshold: threshold,
		Minutes:   minutes,
		Token:     token,
	}
}
