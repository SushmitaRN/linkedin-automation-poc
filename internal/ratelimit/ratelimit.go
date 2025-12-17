package ratelimit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Quotas stores counts per action for a given day
type Quotas map[string]ActionQuota

// ActionQuota stores the date and count for an action
type ActionQuota struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Default storage path
var DefaultQuotaPath = "data/quotas.json"

func ensureDir(path string) error {
	d := filepath.Dir(path)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		return os.MkdirAll(d, 0o755)
	}
	return nil
}

func loadQuotas(path string) (Quotas, error) {
	if path == "" {
		path = DefaultQuotaPath
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Quotas{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var q Quotas
	if err := json.Unmarshal(b, &q); err != nil {
		return nil, err
	}
	return q, nil
}

func saveQuotas(path string, q Quotas) error {
	if path == "" {
		path = DefaultQuotaPath
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// CheckAndIncrement checks whether `action` is under the daily `limit` and increments the counter if allowed.
// Returns an error if the limit has been reached.
func CheckAndIncrement(action string, limit int, path string) error {
	if limit <= 0 {
		return nil
	}
	q, err := loadQuotas(path)
	if err != nil {
		return err
	}
	if q == nil {
		q = Quotas{}
	}
	today := time.Now().Format("2006-01-02")
	aq, ok := q[action]
	if !ok || aq.Date != today {
		// reset
		aq = ActionQuota{Date: today, Count: 0}
	}
	if aq.Count >= limit {
		return fmt.Errorf("daily limit reached for %s (%d)", action, limit)
	}
	aq.Count++
	q[action] = aq
	if err := saveQuotas(path, q); err != nil {
		return err
	}
	return nil
}
