package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/ratelimit"
)

/*
========================
Config & Models
========================
*/

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

/*
========================
Selectors (centralized)
========================
*/

const (
	selectorConnectStatus = "#connect-status"
	selectorMessageBox    = "#message-box"
	selectorSendButton    = "#send-btn"
)

/*
========================
Storage helpers
========================
*/

var messageMu sync.Mutex

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

func saveMessageSafe(path string, msg SentMessage) {
	messageMu.Lock()
	defer messageMu.Unlock()

	arr, _ := loadMessages(path)
	arr = append(arr, msg)
	_ = saveMessages(path, arr)
}

/*
========================
Template rendering
========================
*/

// RenderTemplate replaces {{var}} tokens and errors if unresolved
func RenderTemplate(tpl string, vars map[string]string) (string, error) {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}

	if strings.Contains(out, "{{") {
		return "", errors.New("unresolved template variables")
	}

	return out, nil
}

/*
========================
Public APIs
========================
*/

// SendMessageIfConnected sends a message only if connection is accepted
func SendMessageIfConnected(
	page *rod.Page,
	profileURL string,
	template string,
	vars map[string]string,
	cfg MessageConfig,
) error {
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

	statusEl, err := page.Element(selectorConnectStatus)
	if err != nil || statusEl == nil {
		return fmt.Errorf("connection status not found on %s", profileURL)
	}

	statusText, _ := statusEl.Text()
	statusText = strings.ToLower(statusText)

	if !strings.Contains(statusText, "accepted") &&
		!strings.Contains(statusText, "connected") {
		return errors.New("connection not accepted yet")
	}

	return sendMessageCore(page, profileURL, template, vars, cfg)
}

// SendMessage sends a message without checking connection status
func SendMessage(
	page *rod.Page,
	profileURL string,
	template string,
	vars map[string]string,
	cfg MessageConfig,
) error {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "data/sent_messages.json"
	}

	if err := page.Navigate(profileURL); err != nil {
		return err
	}
	page.MustWaitLoad()

	log.Println("Preparing page for messaging...")
	behavior.ReadingPause()

	return sendMessageCore(page, profileURL, template, vars, cfg)
}

/*
========================
Core send logic
========================
*/

func sendMessageCore(
	page *rod.Page,
	profileURL string,
	template string,
	vars map[string]string,
	cfg MessageConfig,
) error {
	// Check quota (do NOT increment yet)
	if err := ratelimit.Check("message", 5, "data/quotas.json"); err != nil {
		return err
	}

	msg, err := RenderTemplate(template, vars)
	if err != nil {
		return err
	}

	if len(msg) > 500 {
		return errors.New("message too long (max 500 chars)")
	}

	box, err := page.Element(selectorMessageBox)
	if err != nil || box == nil {
		return fmt.Errorf("message box not found on %s", profileURL)
	}

	log.Printf("Typing message (%d chars)...", len(msg))
	if err := behavior.HumanType(box, msg); err != nil {
		return err
	}

	log.Println("Reviewing message...")
	behavior.ReadingPause()

	sendBtn, err := page.Element(selectorSendButton)
	if err != nil || sendBtn == nil {
		return fmt.Errorf("send button not found on %s", profileURL)
	}

	log.Println("Clicking send...")
	if err := sendBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	// Increment quota only after successful send
	if err := ratelimit.Increment("message", "data/quotas.json"); err != nil {
		log.Printf("warning: quota increment failed: %v", err)
	}

	log.Println("✓ Message sent")
	behavior.ReadingPause()

	saveMessageSafe(cfg.StoragePath, SentMessage{
		ProfileURL: profileURL,
		Message:    msg,
		Timestamp:  time.Now(),
	})

	return nil
}
