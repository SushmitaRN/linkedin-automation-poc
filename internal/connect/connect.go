package connect

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/ratelimit"
)

// ConnectConfig controls connect behavior and storage
type ConnectConfig struct {
	DailyLimit  int
	StoragePath string // file path to store sent requests
}

// SentRequest stores a sent connect request record
type SentRequest struct {
	ProfileURL string    `json:"profile_url"`
	Timestamp  time.Time `json:"timestamp"`
}

func ensureStorageDir(path string) error {
	d := filepath.Dir(path)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		return os.MkdirAll(d, 0o755)
	}
	return nil
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
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
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

// PendingMessage represents a queued follow-up message to send after acceptance
type PendingMessage struct {
	ProfileURL string            `json:"profile_url"`
	TemplateID string            `json:"template_id"`
	Vars       map[string]string `json:"vars"`
	EnqueuedAt time.Time         `json:"enqueued_at"`
}

func LoadPending(path string) ([]PendingMessage, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []PendingMessage{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []PendingMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func SavePending(path string, arr []PendingMessage) error {
	if err := ensureStorageDir(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// countToday returns how many connect requests were sent today
func countToday(arr []SentRequest) int {
	now := time.Now()
	c := 0
	for _, s := range arr {
		if s.Timestamp.Year() == now.Year() && s.Timestamp.YearDay() == now.YearDay() {
			c++
		}
	}
	return c
}

// SendConnectRequest navigates to a profile and clicks the mock connect button.
// It respects the daily limit and records sent requests to storage.
func SendConnectRequest(page *rod.Page, profileURL string, cfg ConnectConfig) error {
	if cfg.DailyLimit <= 0 {
		cfg.DailyLimit = 5
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = "data/sent_requests.json"
	}

	// Enforce daily limit via ratelimit module
	if err := ratelimit.CheckAndIncrement("connect", cfg.DailyLimit, "data/quotas.json"); err != nil {
		return err
	}

	arr, err := loadSent(cfg.StoragePath)
	if err != nil {
		return err
	}

	// Navigate to profile
	if err := page.Navigate(profileURL); err != nil {
		return err
	}
	page.MustWaitLoad()
	// Simulate reading profile information
	log.Println("Reading profile details...")
	behavior.ReadingPause()
	behavior.RandomScroll(page)
	behavior.ReadingPause()

	btn, err := page.Element("#connect-btn")
	if err != nil || btn == nil {
		return errors.New("connect button not found on profile")
	}
	log.Println("Hovering over connect button...")
	behavior.ThinkPause()

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	log.Println("✓ Connect request sent")
	behavior.ReadingPause()

	// record
	arr = append(arr, SentRequest{ProfileURL: profileURL, Timestamp: time.Now()})
	if err := saveSent(cfg.StoragePath, arr); err != nil {
		log.Printf("warning: could not save sent requests: %v", err)
	}

	// Enqueue a follow-up message using a default template id.
	// Try to extract first name from profile page for personalization.
	firstName := ""
	if nameEl, err := page.Element("#name"); err == nil && nameEl != nil {
		if txt, err := nameEl.Text(); err == nil {
			parts := strings.Split(txt, " ")
			if len(parts) > 0 {
				firstName = parts[0]
			}
		}
	}
	pendPath := "data/pending_messages.json"
	pend, _ := LoadPending(pendPath)
	pm := PendingMessage{
		ProfileURL: profileURL,
		TemplateID: "welcome_1",
		Vars:       map[string]string{"first_name": firstName},
		EnqueuedAt: time.Now(),
	}
	pend = append(pend, pm)
	if err := SavePending(pendPath, pend); err != nil {
		log.Printf("warning: could not save pending message: %v", err)
	}
	return nil
}
