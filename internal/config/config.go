// Package config discovers and parses the .monitor TOML file into monitors.
//
// It mirrors app/Config/FileFinder.php, app/Monitors/MonitorConfig.php and
// app/Monitors/Monitor.php: the file is searched in the home directory first
// and the working directory second (depth 0), the first match wins, and each
// [monitor-N] section becomes a Monitor.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// FileName is the fixed name of the config file, as in FileFinder.
const FileName = ".monitor"

// rawMonitor is a single TOML section before normalization. threshold is a
// float64 (matching PHP's (float) cast) and minutes defaults to 0 when absent
// (matching Arr::get($monitor, 'minutes', 0)).
type rawMonitor struct {
	Type      string  `toml:"type"`
	Operator  string  `toml:"operator"`
	Threshold float64 `toml:"threshold"`
	Minutes   int     `toml:"minutes"`
	Token     string  `toml:"token"`
}

// DefaultDirectories returns the search path used by the daemon: the user's
// home directory, then the current working directory. This matches
// AppServiceProvider registering FileFinder with home_data_path() and the app
// root, home first.
func DefaultDirectories() []string {
	var dirs []string

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, home)
	}

	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}

	return dirs
}

// Find returns the path of the first .monitor file found in the given
// directories, searching in order. The bool is false when none is found.
func Find(dirs []string) (string, bool) {
	for _, dir := range dirs {
		path := filepath.Join(dir, FileName)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}

	return "", false
}

// Load discovers and parses the .monitor file from the given directories.
// A missing file is not an error: it returns no monitors, matching the PHP
// behavior where an absent config yields an empty monitor set.
func Load(dirs []string) ([]Monitor, error) {
	path, ok := Find(dirs)
	if !ok {
		return nil, nil
	}

	return LoadFile(path)
}

// LoadFile parses a specific .monitor file into monitors.
func LoadFile(path string) ([]Monitor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var sections map[string]rawMonitor
	if err := toml.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	monitors := make([]Monitor, 0, len(sections))
	for key, raw := range sections {
		monitors = append(monitors, newMonitor(
			key,
			raw.Type,
			raw.Operator,
			raw.Threshold,
			raw.Minutes,
			raw.Token,
		))
	}

	return monitors, nil
}
