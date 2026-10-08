package sample

import "golang.org/x/sys/unix"

// Disk samples disk usage for the configured path via statfs, mirroring
// app/Stats/DiskSpace.php (disk_total_space/disk_free_space on "/"). It uses
// Bavail (space available to unprivileged users, as disk_free_space reports),
// and stores used/free as percentages. The block-size factor cancels in the
// percentage, so f_bsize vs f_frsize does not affect the evaluated values.
func (s *Sampler) Disk() (DiskSample, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(s.DiskPath, &st); err != nil {
		return DiskSample{OK: false}, err
	}

	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	free := st.Bavail * bsize

	if total == 0 {
		return DiskSample{OK: false}, nil
	}

	used := total - free
	usedPct := float64(used) / float64(total) * 100
	freePct := float64(free) / float64(total) * 100

	return DiskSample{
		TotalBytes:  int64(total),
		FreePercent: freePct,
		UsedPercent: usedPct,
		OK:          true,
	}, nil
}
