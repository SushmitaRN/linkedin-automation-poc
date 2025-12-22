package main

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/sushmitaRN/linkedin-automation-poc/internal/auth"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/connect"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/message"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/post"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/search"
)

var searchPageURL = "file:///e:/visualstudio/linkedin-automation-poc/mock-site/search.html"

func main() {
	log.Println("Starting LinkedIn automation (Rod)")

	loadDotEnv()

	email := os.Getenv("MOCK_EMAIL")
	password := os.Getenv("MOCK_PASSWORD")

	u := launcher.New().
		Headless(false).
		Leakless(false).
		MustLaunch()

	browser := rod.New().
		ControlURL(u).
		MustConnect()
	defer browser.Close()

	page := browser.MustPage("")
	defer page.Close()

	// 1️⃣ Open login page
	loginURL := "file:///e:/visualstudio/linkedin-automation-poc/mock-site/login.html"
	page.MustNavigate(loginURL)
	page.MustWaitLoad()

	// 2️⃣ Login (this already redirects to search.html)
	if err := auth.Login(page, email, password); err != nil {
		log.Fatalf("Login failed: %v", err)
	}

	// 3️⃣ Wait until search page is ready
	cfg := search.DefaultSearchConfig()
	page.MustWaitLoad()
	page.MustElement(cfg.SearchInputID).MustWaitVisible()

	log.Println("✓ Logged in and search page ready")

	// Reset daily quotas to avoid rate limits during testing
	_ = os.Setenv("DEV_IGNORE_QUOTAS", "1")

	// 4️⃣ Run searches and perform profile connect+message flows

	connCfg := connect.ConnectConfig{
		DailyLimit:  5,
		StoragePath: "data/sent_requests.json",
	}

	runSearchFlow(page, cfg, connCfg, "Bob", "name")
	runSearchFlow(page, cfg, connCfg, "VisionaryAI", "company")
	runSearchFlow(page, cfg, connCfg, "San Francisco", "location")
	runSearchFlow(page, cfg, connCfg, "Engineer", "position")

	log.Println("✓ Automation complete")
}

// ---------------- SEARCH ----------------

func runSearch(page *rod.Page, query string) {
	cfg := search.DefaultSearchConfig()

	log.Printf("Searching for: %s", query)

	input := page.MustElement(cfg.SearchInputID)
	input.MustSelectAllText()
	input.MustInput(query)

	page.MustElement(cfg.SearchButtonID).
		MustClick()

	// Wait for results (NO sleep)
	page.MustWaitElementsMoreThan(cfg.ProfileLinkSel, 0)

	results := page.MustElements(cfg.ProfileLinkSel)
	log.Printf("✓ Found %d results for %q", len(results), query)
}

// normalize takes a possibly-relative href and returns an absolute file:// URL
func normalize(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "file://") {
		return href
	}
	// assume mock-site lives at workspace/mock-site/
	return "file:///e:/visualstudio/linkedin-automation-poc/mock-site/" + href
}

// runSearchFlow performs a search, opens the first profile result, sends connect request and a message
func runSearchFlow(page *rod.Page, cfg search.SearchConfig, connCfg connect.ConnectConfig, query, searchType string) {
	log.Printf("Searching & processing: %s", query)

	// Ensure we're on the search page then perform the search
	if err := page.Navigate(searchPageURL); err != nil {
		log.Printf("warning: could not navigate to search page for %q: %v", query, err)
		return
	}
	page.MustWaitLoad()

	elems, err := search.Search(page, query, cfg)
	if err != nil {
		log.Printf("warning: search failed for %q: %v", query, err)
		return
	}
	if len(elems) == 0 {
		log.Printf("no profiles found for %q", query)
		return
	}
	// Log the first few results for visibility
	maxShow := 5
	if len(elems) < maxShow {
		maxShow = len(elems)
	}
	for i := 0; i < maxShow; i++ {
		t, _ := elems[i].Text()
		log.Printf("  result %d: %s", i+1, strings.TrimSpace(t))
	}

	firstEl := elems[0]
	href := search.ExtractProfileURL(firstEl)
	profURL := normalize(href)
	if profURL == "" {
		log.Printf("no URL for first profile of %q, skipping", query)
		return
	}

	nameText, _ := firstEl.Text()
	nameText = strings.TrimSpace(nameText)
	log.Printf("Opening profile: %s (%s) [type=%s]", nameText, profURL, searchType)

	if searchType == "company" {
		// For company searches, navigate to the company profile page directly
		// Build a company URL e.g. company.html?id=<escaped>
		q := url.QueryEscape(query)
		compURL := normalize("company.html?id=" + q)
		if err := page.Navigate(compURL); err != nil {
			log.Printf("could not navigate to company profile %s: %v", compURL, err)
			return
		}
		page.MustWaitLoad()
		// verify company page loaded
		if el, err := page.Element("#company-name"); err != nil || el == nil {
			log.Printf("warning: company page may not have loaded correctly for %s", query)
		}
	} else {
		// For person/location/position searches, scroll to and click the first profile link
		firstEl.MustWaitVisible()
		firstEl.MustScrollIntoView()
		time.Sleep(250 * time.Millisecond)

		if err := firstEl.Click(proto.InputMouseButtonLeft, 1); err != nil {
			// fallback: navigate directly
			log.Printf("click failed for first result, navigating to %s: %v", profURL, err)
			if err := page.Navigate(profURL); err != nil {
				log.Printf("could not navigate to profile %s: %v", profURL, err)
				return
			}
		}

		page.MustWaitLoad()
		time.Sleep(600 * time.Millisecond)

		// verify person profile loaded
		if el, err := page.Element("#name"); err != nil || el == nil {
			log.Printf("warning: profile page may not have loaded correctly for %s", nameText)
		}
	}

	// Send connect request
	if err := connect.Connect(page, profURL, connCfg); err != nil {
		log.Printf("warning: connect request failed for %s: %v", profURL, err)
	} else {
		log.Printf("✓ Connect request sent to %s", nameText)
	}

	// Send a follow-up message (use a simple template)
	tmpl := "Hi {{first_name}}, thanks for connecting — are there any openings at {{company}}?"
	firstName := strings.Split(nameText, " ")
	fn := ""
	if len(firstName) > 0 {
		fn = firstName[0]
	}

	vars := map[string]string{"first_name": fn, "company": "your company"}
	// try to read company from profile page
	if el, err := page.Element("#company"); err == nil && el != nil {
		if txt, err := el.Text(); err == nil && strings.TrimSpace(txt) != "" {
			vars["company"] = strings.TrimSpace(txt)
		}
	}

	if err := message.SendMessage(page, profURL, tmpl, vars, message.MessageConfig{StoragePath: "data/sent_messages.json"}); err != nil {
		log.Printf("warning: sending message to %s failed: %v", profURL, err)
	} else {
		log.Printf("✓ Message sent to %s", nameText)
	}

	// Like and comment on 1 post after processing each profile.
	// Open a separate tab so we don't disrupt the current page context.
	log.Printf("Interacting with post for: %s", nameText)
	postsPage := page.Browser().MustPage(searchPageURL)
	if postsPage != nil {
		postsPage.MustWaitLoad()
		time.Sleep(500 * time.Millisecond)
		_ = post.InteractWithPosts(postsPage, 1)
		post.HumanScroll(postsPage, 300)
		_ = postsPage.Close()
	} else {
		log.Printf("warning: could not open posts page after processing %s", nameText)
	}

	// small pause between flows
	time.Sleep(800 * time.Millisecond)
}

// ---------------- ENV ----------------

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if os.Getenv(parts[0]) == "" {
			_ = os.Setenv(parts[0], parts[1])
		}
	}
}
