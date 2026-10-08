package sample

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Memory samples memory usage from /proc/meminfo, mirroring
// app/Stats/Memory.php: guarded on readability, MemTotal and MemAvailable read
// in kB, used = total - available, usedPct = used/total*100, freePct =
// 100 - usedPct. The stored `available` column carries the free percentage
// (the PHP quirk), preserved here.
func (s *Sampler) Memory() (MemSample, error) {
	data, err := os.ReadFile(filepath.Join(s.ProcRoot, "meminfo"))
	if err != nil {
		return MemSample{OK: false}, nil
	}

	total := meminfoValue(data, "MemTotal")
	available := meminfoValue(data, "MemAvailable")

	if total == 0 {
		// Avoid division by zero; nothing meaningful to record.
		return MemSample{OK: false}, nil
	}

	used := total - available
	usedPct := float64(used) / float64(total) * 100
	freePct := 100 - usedPct

	return MemSample{
		TotalKB:          total,
		AvailablePercent: freePct,
		UsedPercent:      usedPct,
		FreePercent:      freePct,
		OK:               true,
	}, nil
}

// meminfoValue returns the kB value for a /proc/meminfo key (the second field
// of the matching line), or 0 if absent, matching `grep KEY | awk '{print $2}'`
// followed by an integer cast.
func meminfoValue(data []byte, key string) int64 {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	prefix := key + ":"
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}
