package search

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
)

// SearchConfig holds configuration for search operations
type SearchConfig struct {
	PageURL         string
	SearchInputID   string
	SearchButtonID  string
	ProfileCardSel  string
	ProfileLinkSel  string
	NextButtonID    string
	PrevButtonID    string
	CurrentPageID   string
	TotalPagesID    string
	ProfilesPerPage int
}

// DefaultSearchConfig returns sensible defaults for the mock search page
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		PageURL:         "file:///mock-site/search.html",
		SearchInputID:   "#search-input",
		SearchButtonID:  "#search-btn",
		ProfileCardSel:  ".profile-card",
		ProfileLinkSel:  ".profile-card a",
		NextButtonID:    "#next-btn",
		PrevButtonID:    "#prev-btn",
		CurrentPageID:   "#current-page",
		TotalPagesID:    "#total-pages",
		ProfilesPerPage: 3,
	}
}

// ProfileResult represents a profile found in search results
type ProfileResult struct {
	Name  string
	Title string
	URL   string
}

// SearchResults holds the results of a search operation
type SearchResults struct {
	Profiles      []ProfileResult
	CurrentPage   int
	TotalPages    int
	TotalProfiles int
}

// SearchFirstPage performs a search (single query string) and returns only the first page of results
func SearchFirstPage(page *rod.Page, query string, config SearchConfig) (*SearchResults, error) {
	log.Printf("Starting search (query=%q)", query)

	// Navigate to search page
	err := page.Navigate(config.PageURL)
	if err != nil {
		return nil, err
	}
	page.MustWaitLoad()
	log.Println("Search page loaded")

	// Simulate reading/understanding the page
	log.Println("Reading search page...")
	behavior.ReadingPause()
	behavior.RandomScroll(page)
	behavior.ReadingPause()

	// Fill the single search input with the query
	searchInput, err := page.Element(config.SearchInputID)
	if err != nil || searchInput == nil {
		return nil, errors.New("search input not found")
	}
	_ = searchInput.SelectAllText()
	if err := behavior.HumanType(searchInput, query); err != nil {
		return nil, err
	}

	// Add thinking delay before clicking search
	log.Println("Thinking before search...")
	behavior.ThinkPause()

	// Click search button
	btn, err := page.Element(config.SearchButtonID)
	if err != nil || btn == nil {
		return nil, errors.New("search button not found")
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	log.Println("Search submitted, collecting first page results...")
	behavior.ReadingPause()

	// Wait for client-side JS to render results (mock-site updates DOM)
	// Retry a few times before giving up to handle timing variability.
	for i := 0; i < 10; i++ {
		elems, _ := page.Elements(config.ProfileLinkSel)
		if len(elems) > 0 {
			break
		}
		// also break early if a no-results element is present
		if noRes, _ := page.Element(".no-results"); noRes != nil {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Simulate scanning results
	log.Println("Scanning search results...")
	behavior.ReadingPause()

	// Get total pages
	totalPages := getTotalPages(page, config)

	// Collect profiles from first page
	profiles, err := collectProfilesFromCurrentPage(page, config)
	if err != nil {
		return nil, err
	}

	// Deduplicate results
	deduped := deduplicateProfiles(profiles)
	log.Printf("✓ Found %d profiles on first page (total pages: %d)", len(deduped), totalPages)

	results := &SearchResults{
		Profiles:      deduped,
		CurrentPage:   1,
		TotalPages:    totalPages,
		TotalProfiles: len(deduped),
	}

	return results, nil
}

// NextPage navigates to the next page of search results
func NextPage(page *rod.Page, config SearchConfig) (*SearchResults, error) {
	log.Println("Navigating to next page")
	behavior.ThinkPause()

	// Check if next button is enabled
	nextBtn, err := page.Element(config.NextButtonID)
	if err != nil || nextBtn == nil {
		return nil, errors.New("next button not found")
	}
	if attr, _ := nextBtn.Attribute("disabled"); attr != nil {
		return nil, errors.New("already on last page")
	}

	if err := nextBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	behavior.ReadingPause()
	behavior.RandomScroll(page)
	behavior.ReadingPause()

	// Collect profiles from current page
	profiles, err := collectProfilesFromCurrentPage(page, config)
	if err != nil {
		return nil, err
	}

	currentPage := getCurrentPageNumber(page, config)
	totalPages := getTotalPages(page, config)
	deduped := deduplicateProfiles(profiles)

	log.Printf("✓ Page %d loaded with %d profiles", currentPage, len(deduped))

	results := &SearchResults{
		Profiles:      deduped,
		CurrentPage:   currentPage,
		TotalPages:    totalPages,
		TotalProfiles: len(deduped),
	}

	return results, nil
}

// PreviousPage navigates to the previous page of search results
func PreviousPage(page *rod.Page, config SearchConfig) (*SearchResults, error) {
	log.Println("Navigating to previous page")
	behavior.ThinkPause()

	// Check if prev button is enabled
	prevBtn, err := page.Element(config.PrevButtonID)
	if err != nil || prevBtn == nil {
		return nil, errors.New("prev button not found")
	}
	if attr, _ := prevBtn.Attribute("disabled"); attr != nil {
		return nil, errors.New("already on first page")
	}

	if err := prevBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	behavior.ReadingPause()
	behavior.RandomScroll(page)
	behavior.ReadingPause()

	// Collect profiles from current page
	profiles, err := collectProfilesFromCurrentPage(page, config)
	if err != nil {
		return nil, err
	}

	currentPage := getCurrentPageNumber(page, config)
	totalPages := getTotalPages(page, config)
	deduped := deduplicateProfiles(profiles)

	log.Printf("✓ Page %d loaded with %d profiles", currentPage, len(deduped))

	results := &SearchResults{
		Profiles:      deduped,
		CurrentPage:   currentPage,
		TotalPages:    totalPages,
		TotalProfiles: len(deduped),
	}

	return results, nil
}

// ===== Private Helper Functions =====

// collectProfilesFromCurrentPage extracts profile data from the current page
func collectProfilesFromCurrentPage(page *rod.Page, config SearchConfig) ([]ProfileResult, error) {
	elements, err := page.Elements(config.ProfileLinkSel)
	if err != nil {
		return nil, err
	}

	profiles := make([]ProfileResult, 0, len(elements))

	for _, element := range elements {
		profile := ProfileResult{}

		// Get profile name (link text)
		text, err := element.Text()
		if err != nil {
			log.Printf("Warning: could not get profile name: %v", err)
			continue
		}
		profile.Name = strings.TrimSpace(text)

		// Get profile title (next sibling or nearby element)
		parent, err := element.Parent()
		if err == nil {
			titleElem, err := parent.Element(".profile-title")
			if err == nil {
				titleText, err := titleElem.Text()
				if err == nil {
					profile.Title = strings.TrimSpace(titleText)
				}
			}
		}

		// Get profile URL (href attribute)
		href, err := element.Attribute("href")
		if err == nil && href != nil {
			profile.URL = *href
		}

		// Only add if we have at least a name
		if profile.Name != "" {
			profiles = append(profiles, profile)
		}
	}

	return profiles, nil
}

// getTotalPages extracts the total number of pages from the page
func getTotalPages(page *rod.Page, config SearchConfig) int {
	elem, err := page.Element(config.TotalPagesID)
	if err != nil {
		log.Printf("Warning: could not find total pages element: %v", err)
		return 1
	}

	text, err := elem.Text()
	if err != nil {
		log.Printf("Warning: could not read total pages text: %v", err)
		return 1
	}

	text = strings.TrimSpace(text)

	// Try to parse the string to get the number
	totalPages, err := strconv.Atoi(text)
	if err != nil {
		// If we can't parse, assume at least 1 page
		return 1
	}

	if totalPages < 1 {
		return 1
	}

	return totalPages
}

// getCurrentPageNumber extracts the current page number from the page
func getCurrentPageNumber(page *rod.Page, config SearchConfig) int {
	elem, err := page.Element(config.CurrentPageID)
	if err != nil {
		log.Printf("Warning: could not find current page element: %v", err)
		return 1
	}

	text, err := elem.Text()
	if err != nil {
		log.Printf("Warning: could not read current page text: %v", err)
		return 1
	}

	text = strings.TrimSpace(text)

	// Try to parse the string to get the number
	pageNum, err := strconv.Atoi(text)
	if err != nil {
		return 1
	}

	if pageNum < 1 {
		return 1
	}

	return pageNum
}

// deduplicateProfiles removes duplicate profiles based on name
func deduplicateProfiles(profiles []ProfileResult) []ProfileResult {
	seen := make(map[string]bool)
	deduped := make([]ProfileResult, 0, len(profiles))

	for _, profile := range profiles {
		key := strings.ToLower(profile.Name)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, profile)
		}
	}

	return deduped
}
