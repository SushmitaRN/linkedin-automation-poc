package connect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PendingMessage struct {
	ProfileURL string            `json:"profile_url"`
	TemplateID string            `json:"template_id"`
	Vars       map[string]string `json:"vars"`
	CreatedAt  time.Time         `json:"created_at"`
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o644)
}
