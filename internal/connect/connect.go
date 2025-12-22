package connect

import (
	"encoding/json"

	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/ratelimit"
)

// ConnectConfig controls connect behavior
type ConnectConfig struct {
	DailyLimit  int
	StoragePath string
}

// SentRequest stores a sent connect request record
type SentRequest struct {
	ProfileURL string    `json:"profile_url"`
	Timestamp  time.Time `json:"timestamp"`
}

// ---------------- STORAGE ----------------

func ensureStorageDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func loadSent(path string) ([]SentRequest, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []SentRequest{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []SentRequest
	return arr, json.Unmarshal(b, &arr)
}

func saveSent(path string, arr []SentRequest) error {
	if err := ensureStorageDir(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ---------------- CONNECT ----------------

// Connect assumes the PROFILE PAGE IS ALREADY OPEN
func Connect(page *rod.Page, profileURL string, cfg ConnectConfig) error {
	if cfg.DailyLimit <= 0 {
		cfg.DailyLimit = 5
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = "data/sent_requests.json"
	}

	// Rate limit
	if err := ratelimit.CheckAndIncrement("connect", cfg.DailyLimit, "data/quotas.json"); err != nil {
		return err
	}

	// Ensure connect button exists
	btn := page.MustElement("#connect-btn")
	btn.MustWaitVisible()
	btn.MustScrollIntoView()

	behavior.ThinkPause()

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	log.Println("✓ Connect clicked")

	// Optional confirmation element
	page.MustWaitIdle()

	// Record connect
	arr, _ := loadSent(cfg.StoragePath)
	arr = append(arr, SentRequest{
		ProfileURL: profileURL,
		Timestamp:  time.Now(),
	})
	if err := saveSent(cfg.StoragePath, arr); err != nil {
		log.Printf("warning: could not save sent request: %v", err)
	}

	return nil
}
