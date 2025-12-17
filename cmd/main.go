package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/auth"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/connect"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/message"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/scheduler"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/search"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/templates"
)

func main() {
	log.Println("Starting browser automation with mock site")

	// Read required inputs from environment
	// Load .env if present and ensure required env vars exist (fallback to safe defaults)
	loadDotEnv()

	// Read inputs from environment (defaults applied by loadDotEnv)
	email := os.Getenv("MOCK_EMAIL")
	password := os.Getenv("MOCK_PASSWORD")
	searchQuery := os.Getenv("SEARCH_QUERY")
	if email == "" || password == "" {
		// last-resort defaults
		email = "test@example.com"
		password = "password123"
		log.Println("Warning: using default MOCK_EMAIL/MOCK_PASSWORD; set env vars to override")
	}
	if searchQuery == "" {
		// default: empty search (page will show featured posts)
		log.Println("Warning: no SEARCH_QUERY set; page will show featured content. Set SEARCH_QUERY to search.")
	}

	// Launch browser
	u := launcher.New().
		Headless(false).
		Leakless(false).
		MustLaunch()

	browser := rod.New().
		ControlURL(u).
		MustConnect()
	defer browser.MustClose()

	// Open a page and navigate to mock login
	page := browser.MustPage("")
	defer page.MustClose()

	loginURL := "file:///e:/visualstudio/linkedin-automation-poc/mock-site/login.html"
	if err := page.Navigate(loginURL); err != nil {
		log.Fatalf("could not open login page: %v", err)
	}
	page.MustWaitLoad()

	// Attempt to load saved cookies (best-effort) then perform login (do not skip login)
	_ = auth.LoadCookies(page, "data/session.cookie")
	if err := auth.Login(page, email, password); err != nil {
		log.Fatalf("login failed: %v", err)
	}
	// Save session cookies after successful login
	_ = auth.SaveCookies(page, "data/session.cookie")

	// Get default search configuration and set proper local file URL
	config := search.DefaultSearchConfig()
	config.PageURL = "file:///e:/visualstudio/linkedin-automation-poc/mock-site/search.html"

	// Navigate to search page
	if err := page.Navigate(config.PageURL); err != nil {
		log.Fatalf("could not open search page: %v", err)
	}
	page.MustWaitLoad()

	// Perform search (first page)
	log.Println("\n=== Performing search ===")
	// prefer name+location if provided
	results, err := search.SearchFirstPage(page, searchQuery, config)
	if err != nil {
		log.Fatalf("search failed: %v", err)
	}

	log.Printf("Found %d profiles on page 1 of %d\n", len(results.Profiles), results.TotalPages)
	for i, profile := range results.Profiles {
		log.Printf("  %d. %s - %s", i+1, profile.Name, profile.Title)
	}

	// Send connect requests for discovered profiles (respecting daily limit)
	connCfg := connect.ConnectConfig{DailyLimit: 5, StoragePath: "data/sent_requests.json"}
	for _, p := range results.Profiles {
		profileURL := p.URL
		if profileURL == "" {
			log.Printf("warning: %s has no URL, skipping connect", p.Name)
			continue
		}
		// Convert relative URLs to absolute file:// URLs
		if !strings.HasPrefix(profileURL, "file://") && !strings.HasPrefix(profileURL, "http") {
			profileURL = "file:///e:/visualstudio/linkedin-automation-poc/mock-site/" + profileURL
		}
		if err := connect.SendConnectRequest(page, profileURL, connCfg); err != nil {
			log.Printf("connect skipped for %s: %v", p.Name, err)
		} else {
			log.Printf("connect sent to %s", p.Name)
		}
	}

	// Try to send follow-up messages for accepted connections (limited per run)
	// Load templates and pick one by ID (default to first)
	templatesList, err := templates.LoadTemplates("")
	if err != nil || len(templatesList) == 0 {
		log.Printf("warning: could not load templates: %v", err)
	}
	var tpl *templates.Template
	if len(templatesList) > 0 {
		tpl = &templatesList[0]
	}

	msgCfg := message.MessageConfig{StoragePath: "data/sent_messages.json"}
	msgsSent := 0
	msgsLimit := 3
	for _, p := range results.Profiles {
		if msgsSent >= msgsLimit {
			break
		}
		profileURL := p.URL
		if profileURL == "" {
			continue
		}
		// Convert relative URLs to absolute file:// URLs
		if !strings.HasPrefix(profileURL, "file://") && !strings.HasPrefix(profileURL, "http") {
			profileURL = "file:///e:/visualstudio/linkedin-automation-poc/mock-site/" + profileURL
		}
		if tpl == nil {
			continue
		}
		vars := map[string]string{"first_name": strings.Split(p.Name, " ")[0], "company": ""}
		if err := message.SendMessageIfConnected(page, profileURL, tpl.Body, vars, msgCfg); err != nil {
			log.Printf("message skipped for %s: %v", p.Name, err)
			continue
		}
		msgsSent++
		log.Printf("message sent to %s", p.Name)
	}

	// Process any pending messages (scheduler)
	schedCfg := scheduler.SchedulerConfig{PendingPath: "data/pending_messages.json", TemplatesPath: "", MsgStorage: "data/sent_messages.json"}
	if err := scheduler.ProcessPending(page, schedCfg); err != nil {
		log.Printf("scheduler error: %v", err)
	}

	// Optionally navigate to next page (demonstration, with short timeout)
	if results.TotalPages > 1 {
		time.Sleep(1 * time.Second)
		log.Println("\n=== Navigating to next page ===")

		// Create a context with a short timeout for page navigation
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		page = page.Context(ctx)
		nextResults, err := search.NextPage(page, config)
		if err != nil {
			log.Printf("Warning: could not navigate to next page: %v", err)
		} else {
			log.Printf("Found %d profiles on page %d of %d\n", len(nextResults.Profiles), nextResults.CurrentPage, nextResults.TotalPages)
			for i, profile := range nextResults.Profiles {
				log.Printf("  %d. %s - %s", i+1, profile.Name, profile.Title)
			}
		}
	}

	log.Println("\n=== Automation complete ===")
}

// loadDotEnv reads a .env file in the workspace root (if present) and sets any missing env vars.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		// no .env — try to pick up editor-provided env (do nothing)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// remove optional surrounding quotes
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = strings.Trim(val, "\"")
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
			log.Printf("Loaded env %s from .env", key)
		}
	}
}
