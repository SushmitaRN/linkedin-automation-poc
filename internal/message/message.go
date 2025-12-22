package message

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

// MessageConfig controls messaging behavior and storage
type MessageConfig struct {
	StoragePath string
}

// SentMessage record
type SentMessage struct {
	ProfileURL string    `json:"profile_url"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

func ensureDir(path string) error {
	d := filepath.Dir(path)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		return os.MkdirAll(d, 0o755)
	}
	return nil
}

func loadMessages(path string) ([]SentMessage, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []SentMessage{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []SentMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func saveMessages(path string, arr []SentMessage) error {
	if err := ensureDir(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// RenderTemplate replaces {{var}} tokens in template with values from vars map
func RenderTemplate(tpl string, vars map[string]string) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

// SendMessageIfConnected navigates to a profile, checks connection status, and sends a message if connected
func SendMessageIfConnected(page *rod.Page, profileURL, template string, vars map[string]string, cfg MessageConfig) error {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "data/sent_messages.json"
	}

	if err := page.Navigate(profileURL); err != nil {
		return err
	}
	page.MustWaitLoad()
	log.Println("Reading profile for messaging...")
	behavior.ReadingPause()
	behavior.RandomScroll(page)
	behavior.ReadingPause()

	// Check connection status element
	statusEl, err := page.Element("#connect-status")
	if err != nil || statusEl == nil {
		return errors.New("could not find connection status on profile")
	}
	statusText, _ := statusEl.Text()
	if !strings.Contains(strings.ToLower(statusText), "accepted") && !strings.Contains(strings.ToLower(statusText), "connected") {
		// not accepted yet
		return errors.New("connection not accepted yet")
	}

	log.Println("Connection accepted! Composing message...")
	behavior.ThinkPause()

	// Enforce message daily quota
	if err := ratelimit.CheckAndIncrement("message", 5, "data/quotas.json"); err != nil {
		return err
	}

	// Compose message
	msg := RenderTemplate(template, vars)

	box, err := page.Element("#message-box")
	if err != nil || box == nil {
		return errors.New("message box not found on profile")
	}
	log.Printf("Typing message (%d chars)...", len(msg))
	if err := behavior.HumanType(box, msg); err != nil {
		return err
	}

	log.Println("Message typed. Reviewing before send...")
	behavior.ReadingPause()

	sendBtn, err := page.Element("#send-btn")
	if err != nil || sendBtn == nil {
		return errors.New("send button not found")
	}
	log.Println("Clicking send...")
	if err := sendBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	log.Println("✓ Message sent")
	behavior.ReadingPause()

	arr, _ := loadMessages(cfg.StoragePath)
	arr = append(arr, SentMessage{ProfileURL: profileURL, Message: msg, Timestamp: time.Now()})
	if err := saveMessages(cfg.StoragePath, arr); err != nil {
		log.Printf("warning: could not save messages: %v", err)
	}

	return nil
}

// SendMessage sends a message to the given profile/page without checking connection status.
func SendMessage(page *rod.Page, profileURL, template string, vars map[string]string, cfg MessageConfig) error {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "data/sent_messages.json"
	}

	if err := page.Navigate(profileURL); err != nil {
		return err
	}
	page.MustWaitLoad()
	log.Println("Preparing page for messaging...")
	behavior.ReadingPause()

	// Enforce message daily quota
	if err := ratelimit.CheckAndIncrement("message", 5, "data/quotas.json"); err != nil {
		return err
	}

	// Compose message
	msg := RenderTemplate(template, vars)

	box, err := page.Element("#message-box")
	if err != nil || box == nil {
		return errors.New("message box not found on page")
	}
	log.Printf("Typing message (%d chars)...", len(msg))
	if err := behavior.HumanType(box, msg); err != nil {
		return err
	}

	behavior.ReadingPause()

	sendBtn, err := page.Element("#send-btn")
	if err != nil || sendBtn == nil {
		return errors.New("send button not found")
	}
	if err := sendBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	log.Println("✓ Message sent (no-connection check)")
	behavior.ReadingPause()

	arr, _ := loadMessages(cfg.StoragePath)
	arr = append(arr, SentMessage{ProfileURL: profileURL, Message: msg, Timestamp: time.Now()})
	if err := saveMessages(cfg.StoragePath, arr); err != nil {
		log.Printf("warning: could not save messages: %v", err)
	}

	return nil
}
