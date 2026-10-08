package daemon

import (
	"net"
	"os"
	"strconv"
	"time"
)

// sdNotify sends a service-manager notification via $NOTIFY_SOCKET. It is a
// no-op when the socket is unset (i.e. not running under systemd Type=notify),
// so it is safe on any platform. This avoids a dependency on go-systemd.
func sdNotify(state string) error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return nil
	}

	name := socket
	// Abstract-namespace sockets are encoded with a leading '@'.
	if name[0] == '@' {
		name = "\x00" + name[1:]
	}

	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: name, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}

// watchdogInterval returns how often WATCHDOG=1 should be sent, derived from
// systemd's WATCHDOG_USEC (half the timeout, per the sd_watchdog convention).
// It returns 0 when the watchdog is not enabled for this process.
func watchdogInterval() time.Duration {
	usec := os.Getenv("WATCHDOG_USEC")
	if usec == "" {
		return 0
	}

	micros, err := strconv.ParseInt(usec, 10, 64)
	if err != nil || micros <= 0 {
		return 0
	}

	return time.Duration(micros) * time.Microsecond / 2
}
