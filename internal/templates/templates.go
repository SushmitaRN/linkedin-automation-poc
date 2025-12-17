package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Template represents a message template with an ID and content
type Template struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	DailyLimit int    `json:"daily_limit"`
}

// LoadTemplates reads templates from a JSON file
func LoadTemplates(path string) ([]Template, error) {
	if path == "" {
		path = "data/templates.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []Template
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

// GetTemplateByID returns the template with matching id or nil if not found
func GetTemplateByID(templates []Template, id string) *Template {
	for _, t := range templates {
		if t.ID == id {
			copy := t
			return &copy
		}
	}
	return nil
}

// EnsureTemplatesDir ensures the data directory exists for storing templates
func EnsureTemplatesDir(path string) error {
	if path == "" {
		path = "data/templates.json"
	}
	d := filepath.Dir(path)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		return os.MkdirAll(d, 0o755)
	}
	return nil
}
