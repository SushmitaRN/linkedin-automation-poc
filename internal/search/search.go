package search

import (
	"errors"
	"log"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
)

// SearchConfig holds configuration for search operations
type SearchConfig struct {
	SearchInputID  string
	SearchButtonID string
	ProfileLinkSel string
}

// DefaultSearchConfig returns sensible defaults for the mock search page
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		SearchInputID:  "#search-input",
		SearchButtonID: "#search-btn",
		ProfileLinkSel: ".profile-card a",
	}
}

// Search performs search on CURRENT page and returns profile links
func Search(page *rod.Page, query string, cfg SearchConfig) ([]*rod.Element, error) {
	log.Printf("Searching for %q", query)

	// Ensure search input is visible
	input := page.MustElement(cfg.SearchInputID)
	input.MustWaitVisible()

	// Type query
	input.MustSelectAllText()
	if err := behavior.HumanType(input, query); err != nil {
		return nil, err
	}

	behavior.ThinkPause()

	// Click search
	btn := page.MustElement(cfg.SearchButtonID)
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	// Wait for results
	page.MustWaitElementsMoreThan(cfg.ProfileLinkSel, 0)

	results := page.MustElements(cfg.ProfileLinkSel)
	if len(results) == 0 {
		return nil, errors.New("no profiles found")
	}

	log.Printf("✓ Found %d profiles for %q", len(results), query)
	return results, nil
}

// ExtractProfileURL safely extracts href
func ExtractProfileURL(el *rod.Element) string {
	href, err := el.Attribute("href")
	if err != nil || href == nil {
		return ""
	}
	return strings.TrimSpace(*href)
}
