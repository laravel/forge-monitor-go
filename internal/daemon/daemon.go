// Package daemon runs the sample/evaluate cycle on a minute-aligned schedule,
// reloading config each cycle, pruning daily, and integrating with the systemd
// watchdog. The run loop never touches systemctl; installation is separate.
package daemon

import (
	"context"
	"time"

	"log/slog"

	"github.com/laravel/forge-monitor-go/internal/config"
	"github.com/laravel/forge-monitor-go/internal/eval"
	"github.com/laravel/forge-monitor-go/internal/sample"
	"github.com/laravel/forge-monitor-go/internal/store"
)

// retentionDays matches CleanupCommand's 7-day cutoff.
const retentionDays = 7

// pruneEvery is how often old samples are pruned. The 7-day cutoff is the
// invariant; the cadence is cosmetic.
const pruneEvery = 24 * time.Hour

// cycleTimeoutFloor bounds a single cycle even when the interval is short, so a
// stuck dependency cannot wedge the loop (and, under systemd, the missed
// watchdog ping triggers a restart).
const cycleTimeoutFloor = 30 * time.Second

// Daemon owns one run of the monitor loop.
type Daemon struct {
	store     *store.Store
	evaluator *eval.Evaluator
	sampler   *sample.Sampler
	dirs      []string
	interval  time.Duration
	log       *slog.Logger
}

// New builds a Daemon. dirs is the .monitor search path; interval is the sample
// period (60s in production).
func New(s *store.Store, ev *eval.Evaluator, sm *sample.Sampler, dirs []string, interval time.Duration, log *slog.Logger) *Daemon {
	return &Daemon{
		store:     s,
		evaluator: ev,
		sampler:   sm,
		dirs:      dirs,
		interval:  interval,
		log:       log,
	}
}

// Run executes the loop until ctx is cancelled (SIGINT/SIGTERM). It aligns each
// cycle to the next interval boundary, prunes daily, and pings the systemd
// watchdog after each successful cycle.
func (d *Daemon) Run(ctx context.Context) error {
	if err := sdNotify("READY=1"); err != nil {
		d.log.Warn("sd_notify READY failed", "error", err)
	}
	if iv := watchdogInterval(); iv > 0 {
		d.log.Info("systemd watchdog enabled", "ping_interval", iv.String())
	}

	d.prune()
	lastPrune := time.Now()

	for {
		next := time.Now().Truncate(d.interval).Add(d.interval)
		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			timer.Stop()
			d.log.Info("shutting down")
			return nil
		case <-timer.C:
		}

		d.RunOnce(ctx)

		if err := sdNotify("WATCHDOG=1"); err != nil {
			d.log.Warn("sd_notify WATCHDOG failed", "error", err)
		}

		if time.Since(lastPrune) >= pruneEvery {
			d.prune()
			lastPrune = time.Now()
		}
	}
}

// RunOnce performs a single cycle: reload config, then for each present
// category sample once and evaluate its monitors. Errors are logged, never
// fatal, so one bad monitor or a transient failure does not abort the cycle.
func (d *Daemon) RunOnce(parent context.Context) {
	timeout := d.interval
	if timeout < cycleTimeoutFloor {
		timeout = cycleTimeoutFloor
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	monitors, err := config.Load(d.dirs)
	if err != nil {
		d.log.Warn("failed to load .monitor config", "error", err)
		return
	}
	if len(monitors) == 0 {
		return
	}

	byCategory := map[string][]config.Monitor{}
	for _, m := range monitors {
		byCategory[m.Category()] = append(byCategory[m.Category()], m)
	}

	// Sample-then-evaluate per category, in the same order as the PHP kernel
	// (disk, load, memory). A category with no monitors is not sampled.
	if ms := byCategory[config.CategoryDisk]; len(ms) > 0 {
		d.sampleDisk()
		d.evaluateAll(ctx, ms)
	}
	if ms := byCategory[config.CategoryLoad]; len(ms) > 0 {
		d.sampleLoad()
		d.evaluateAll(ctx, ms)
	}
	if ms := byCategory[config.CategoryMemory]; len(ms) > 0 {
		d.sampleMemory()
		d.evaluateAll(ctx, ms)
	}
}

func (d *Daemon) evaluateAll(ctx context.Context, monitors []config.Monitor) {
	for _, m := range monitors {
		d.evaluateSafe(ctx, m)
	}
}

func (d *Daemon) evaluateSafe(ctx context.Context, m config.Monitor) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("monitor evaluation panicked", "monitor", m.Key, "panic", r)
		}
	}()

	if err := d.evaluator.Evaluate(ctx, m); err != nil {
		d.log.Error("monitor evaluation failed", "monitor", m.Key, "type", m.Type, "error", err)
	}
}

func (d *Daemon) sampleDisk() {
	s, err := d.sampler.Disk()
	if err != nil {
		d.log.Warn("disk sample failed", "error", err)
		return
	}
	if !s.OK {
		return
	}
	if err := d.store.InsertDisk(s.TotalBytes, s.FreePercent, s.UsedPercent); err != nil {
		d.log.Error("failed to store disk sample", "error", err)
	}
}

func (d *Daemon) sampleLoad() {
	s, err := d.sampler.Load()
	if err != nil {
		d.log.Warn("load sample failed", "error", err)
		return
	}
	if !s.OK {
		return
	}
	if err := d.store.InsertLoadAvg(s.LoadAvg, s.LoadAvgPercent, s.Cpus); err != nil {
		d.log.Error("failed to store load sample", "error", err)
	}
}

func (d *Daemon) sampleMemory() {
	s, err := d.sampler.Memory()
	if err != nil {
		d.log.Warn("memory sample failed", "error", err)
		return
	}
	if !s.OK {
		return
	}
	if err := d.store.InsertMemory(s.TotalKB, s.AvailablePercent, s.UsedPercent, s.FreePercent); err != nil {
		d.log.Error("failed to store memory sample", "error", err)
	}
}

func (d *Daemon) prune() {
	if err := d.store.Prune(retentionDays); err != nil {
		d.log.Warn("prune failed", "error", err)
	}
}
