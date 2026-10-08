package sample

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Load samples the 1-minute load average and its percentage of cores. It
// mirrors app/Stats/LoadAvg.php: guarded on /proc/cpuinfo readability, cores
// counted from /proc/cpuinfo "processor" lines (not runtime.NumCPU, which
// honors cgroup limits), 1-minute load from /proc/loadavg, and
// load_avg_percent = round(load / max(1, cores) * 100, 2).
func (s *Sampler) Load() (LoadSample, error) {
	cpuinfo, err := os.ReadFile(filepath.Join(s.ProcRoot, "cpuinfo"))
	if err != nil {
		// Unreadable /proc/cpuinfo: skip the sample, as the PHP guard does.
		return LoadSample{OK: false}, nil
	}

	cores := countProcessors(cpuinfo)

	load, err := readLoadAvg1(filepath.Join(s.ProcRoot, "loadavg"))
	if err != nil {
		return LoadSample{OK: false}, nil
	}

	denom := cores
	if denom < 1 {
		denom = 1
	}
	percent := round2(load / float64(denom) * 100)

	return LoadSample{
		LoadAvg:        load,
		LoadAvgPercent: percent,
		Cpus:           int64(cores),
		OK:             true,
	}, nil
}

// countProcessors counts lines beginning with "processor", matching
// `grep "^processor" /proc/cpuinfo | wc -l`.
func countProcessors(cpuinfo []byte) int {
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(cpuinfo))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	return count
}

// readLoadAvg1 reads the first whitespace-separated field of /proc/loadavg.
func readLoadAvg1(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, strconv.ErrSyntax
	}

	return strconv.ParseFloat(fields[0], 64)
}

// round2 rounds to two decimals using half-away-from-zero, matching PHP round().
func round2(x float64) float64 {
	return math.Round(x*100) / 100
}
