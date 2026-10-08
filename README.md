# Forge Monitor (Go)

A single static Go binary that replaces the PHP/Laravel `forge-monitor` agent. It samples
CPU load, memory, and disk once a minute, stores samples in the same local SQLite database,
evaluates each configured monitor against its threshold, and POSTs to Forge when a monitor
changes state (OK ↔ ALERT).

It is a **drop-in replacement**: it reads and writes the same `database.sqlite` schema, the
same `.monitor` TOML config format, and the same `{monitor, token, state}` alert contract as
the PHP version.

## Running

```bash
forge-monitor --database /path/to/database.sqlite          # long-running daemon
forge-monitor --once --database /path/to/database.sqlite    # single cycle then exit
```

On startup the daemon creates the SQLite database (and its parent directory) if
it does not already exist, otherwise it uses the existing one.

## Running as a service

The daemon has no install command. Forge provisioning drops the systemd unit.
See `packaging/forge-monitor.service` for the expected unit; `Type=notify` plus
`WatchdogSec` enable the systemd watchdog so a hung daemon is restarted.

## Config (`.monitor`)

TOML file named `.monitor` in the user's home directory or the working directory. Example:

```toml
[monitor-1]
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
```

Monitor types: `cpu_load`, `disk`, `free_memory`, `used_memory`. Operators: `gte`, `lte`.
`minutes` is the consecutive-breach window; it is ignored for `disk` (forced to 1).
