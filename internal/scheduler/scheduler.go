package scheduler

import (
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/connect"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/message"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/templates"
)

// Note: pending messages are stored in data/pending_messages.json
// This scheduler will attempt to send pending messages using the message module.

type SchedulerConfig struct {
	PendingPath   string
	TemplatesPath string
	MsgStorage    string
}

// ProcessPending loads pending messages and attempts to send them.
// Successfully sent messages are removed from the pending queue.
func ProcessPending(page *rod.Page, cfg SchedulerConfig) error {
	if cfg.PendingPath == "" {
		cfg.PendingPath = "data/pending_messages.json"
	}
	if cfg.TemplatesPath == "" {
		cfg.TemplatesPath = ""
	}
	if cfg.MsgStorage == "" {
		cfg.MsgStorage = "data/sent_messages.json"
	}

	// Load templates
	tpls, _ := templates.LoadTemplates(cfg.TemplatesPath)

	// Load pending messages
	pend, err := connect.LoadPending(cfg.PendingPath)
	if err != nil {
		return err
	}

	remaining := make([]connect.PendingMessage, 0, len(pend))

	for _, pm := range pend {
		// find template body
		var body string
		for _, t := range tpls {
			if t.ID == pm.TemplateID {
				body = t.Body
				break
			}
		}
		if body == "" {
			log.Printf("template %s not found, skipping message to %s", pm.TemplateID, pm.ProfileURL)
			remaining = append(remaining, pm)
			continue
		}

		// attempt to send
		err := message.SendMessageIfConnected(page, pm.ProfileURL, body, pm.Vars, message.MessageConfig{StoragePath: cfg.MsgStorage})
		if err != nil {
			log.Printf("pending message not sent to %s: %v", pm.ProfileURL, err)
			remaining = append(remaining, pm)
		} else {
			log.Printf("pending message sent to %s", pm.ProfileURL)
		}

		// wait a bit between messages
		behavior.SleepHuman(800*time.Millisecond, 1500*time.Millisecond)
	}

	// Save remaining pending messages
	if err := connect.SavePending(cfg.PendingPath, remaining); err != nil {
		log.Printf("warning: could not save pending messages: %v", err)
	}

	return nil
}
