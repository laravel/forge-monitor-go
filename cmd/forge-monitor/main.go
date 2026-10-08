// Command forge-monitor is a drop-in Go replacement for the PHP forge-monitor
// agent. It samples CPU load, memory and disk each minute, stores samples in
// the same SQLite database, evaluates configured monitors, and pings Forge on
// state changes.
//
// It is a plain daemon: running it runs the monitor loop. There are no
// subcommands. Forge provisioning installs the systemd unit (see
// packaging/forge-monitor.service for the expected unit).
//
//	forge-monitor [--database PATH] [--config-dir DIR ...] [--endpoint URL] [--interval D] [--once]
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/laravel/forge-monitor-go/internal/config"
	"github.com/laravel/forge-monitor-go/internal/daemon"
	"github.com/laravel/forge-monitor-go/internal/eval"
	"github.com/laravel/forge-monitor-go/internal/notify"
	"github.com/laravel/forge-monitor-go/internal/sample"
	"github.com/laravel/forge-monitor-go/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var dirs stringSlice
	database := flag.String("database", envOr("DB_DATABASE", defaultDatabase()), "SQLite database path")
	endpoint := flag.String("endpoint", os.Getenv("MONITOR_ENDPOINT"), "Alert endpoint URL")
	interval := flag.Duration("interval", time.Minute, "Sample interval")
	once := flag.Bool("once", false, "Run a single cycle and exit")
	flag.Var(&dirs, "config-dir", "Directory to search for .monitor (repeatable; default: home, cwd)")
	flag.Parse()

	searchDirs := []string(dirs)
	if len(searchDirs) == 0 {
		searchDirs = config.DefaultDirectories()
	}

	st, err := store.Open(*database, log)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	notifier := notify.New(*endpoint)
	evaluator := eval.NewEvaluator(st, notifier, log)
	sampler := sample.New()
	d := daemon.New(st, evaluator, sampler, searchDirs, *interval, log)

	if *once {
		d.RunOnce(context.Background())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("starting forge-monitor",
		"database", *database, "endpoint", notifier.Endpoint(), "interval", interval.String())

	if err := d.Run(ctx); err != nil {
		log.Error("daemon exited with error", "error", err)
		os.Exit(1)
	}
}

func defaultDatabase() string {
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "database", "database.sqlite")
	}
	return filepath.Join("database", "database.sqlite")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// stringSlice collects repeated string flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}
