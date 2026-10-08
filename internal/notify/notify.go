// Package notify sends monitor state changes to Forge, matching
// app/Notifier.php: an application/json POST of {monitor, token, state}.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/laravel/forge-monitor-go/internal/config"
)

// DefaultEndpoint is used when no endpoint is configured, matching
// config/monitor.php.
const DefaultEndpoint = "https://forge.laravel.com/monitors/ping"

// Notifier posts alerts to a Forge endpoint.
type Notifier struct {
	endpoint string
	client   *http.Client
}

// New returns a Notifier for the given endpoint. An empty endpoint falls back
// to DefaultEndpoint (a deliberate divergence from Laravel's env(), which would
// treat a set-but-empty value as ""; the daemon always targets Forge).
func New(endpoint string) *Notifier {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	return &Notifier{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Endpoint returns the resolved endpoint URL.
func (n *Notifier) Endpoint() string {
	return n.endpoint
}

// Alert posts the monitor's new state. Errors are returned for logging but,
// like the PHP notifier, they do not roll back the alert row that was already
// written; callers should log and continue.
func (n *Notifier) Alert(ctx context.Context, m config.Monitor, state string) error {
	body, err := json.Marshal(map[string]string{
		"monitor": m.Key,
		"token":   m.Token,
		"state":   state,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("monitor ping returned status %d", resp.StatusCode)
	}

	return nil
}
