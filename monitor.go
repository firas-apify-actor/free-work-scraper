package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	monitorStore = "free-work-monitor-state"
	maxSeen      = 50000
	slackMaxLine = 20
)

var stateKeyRe = regexp.MustCompile(`^[a-zA-Z0-9!\-_.'()]{1,100}$`)

type seenEntry struct {
	ID int    `json:"id"`
	At string `json:"at"`
}

// Monitor remembers which mission ids were already returned (oldest first, capped).
type Monitor struct {
	key   string
	Seen  []seenEntry `json:"seen"`
	index map[int]string
}

func LoadMonitor(c *Client, key string) (*Monitor, error) {
	if !stateKeyRe.MatchString(key) {
		return nil, fmt.Errorf("invalid stateKey %q: use letters, digits and !-_.'() only", key)
	}
	m := &Monitor{key: key, index: map[int]string{}}
	if _, err := c.GetValue(monitorStore, key, m); err != nil {
		return nil, fmt.Errorf("load monitor state: %w", err)
	}
	for _, e := range m.Seen {
		m.index[e.ID] = e.At
	}
	return m, nil
}

// FirstSeen returns when id was first returned, if ever.
func (m *Monitor) FirstSeen(id int) (string, bool) { at, ok := m.index[id]; return at, ok }

func (m *Monitor) Add(id int, at string) {
	if _, ok := m.index[id]; ok {
		return
	}
	m.index[id] = at
	m.Seen = append(m.Seen, seenEntry{id, at})
}

func (m *Monitor) Save(c *Client) error {
	if over := len(m.Seen) - maxSeen; over > 0 {
		m.Seen = m.Seen[over:]
	}
	return c.SetValue(monitorStore, m.key, m)
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func rateText(m Mission) string {
	switch {
	case m.DailyRateMin != nil && m.DailyRateMax != nil && *m.DailyRateMin != *m.DailyRateMax:
		return fmt.Sprintf("%.0f-%.0f €/j", *m.DailyRateMin, *m.DailyRateMax)
	case m.DailyRateMax != nil:
		return fmt.Sprintf("%.0f €/j", *m.DailyRateMax)
	case m.DailyRateMin != nil:
		return fmt.Sprintf("%.0f €/j", *m.DailyRateMin)
	}
	return "TJM n/a"
}

// slackText builds one compact message of at most slackMaxLine lines.
func slackText(ms []Mission) string {
	lines := []string{fmt.Sprintf("*%d new Free-Work mission(s)*", len(ms))}
	room := slackMaxLine - 1
	show := ms
	if len(ms) > room {
		show, room = ms[:room-1], room-1
	}
	for _, m := range show {
		lines = append(lines, fmt.Sprintf("• <%s|%s> — %s · %s · %s", m.URL, esc(m.Title), rateText(m), m.RemoteMode, esc(m.Location)))
	}
	if len(show) < len(ms) {
		lines = append(lines, fmt.Sprintf("…and %d more", len(ms)-len(show)))
	}
	return strings.Join(lines, "\n")
}

// sendSlack posts one message. The webhook URL is a secret: never include it in errors.
func sendSlack(ctx context.Context, webhook string, ms []Mission) error {
	body, _ := json.Marshal(map[string]string{"text": slackText(ms)})
	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid slackWebhookUrl")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("slack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("slack: HTTP %d", resp.StatusCode)
	}
	return nil
}
