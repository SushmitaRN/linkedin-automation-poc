package ratelimit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

var mu sync.Mutex

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

/*
========================
NEW: Check (no increment)
========================
*/

// Check verifies quota without incrementing
func Check(action string, limit int, path string) error {
	if os.Getenv("DEV_IGNORE_QUOTAS") == "1" {
		return nil
	}
	if limit <= 0 {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	q, err := loadQuotas(path)
	if err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	aq, ok := q[action]
	if !ok || aq.Date != today {
		return nil // zero usage today
	}

	if aq.Count >= limit {
		return fmt.Errorf("daily limit reached for %s (%d)", action, limit)
	}

	return nil
}

/*
========================
NEW: Increment only
========================
*/

// Increment increments quota after success
func Increment(action string, path string) error {
	if os.Getenv("DEV_IGNORE_QUOTAS") == "1" {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

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
		aq = ActionQuota{Date: today, Count: 0}
	}

	aq.Count++
	q[action] = aq

	return saveQuotas(path, q)
}

/*
========================
EXISTING: CheckAndIncrement
(unchanged behavior)
========================
*/

// CheckAndIncrement checks whether `action` is under the daily `limit` and increments the counter if allowed.
func CheckAndIncrement(action string, limit int, path string) error {
	if os.Getenv("DEV_IGNORE_QUOTAS") == "1" {
		return nil
	}
	if limit <= 0 {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

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
		aq = ActionQuota{Date: today, Count: 0}
	}

	if aq.Count >= limit {
		return fmt.Errorf("daily limit reached for %s (%d)", action, limit)
	}

	aq.Count++
	q[action] = aq

	return saveQuotas(path, q)
}
