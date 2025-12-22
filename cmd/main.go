package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/auth"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/connect"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/message"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/post"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/ratelimit"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/scheduler"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/search"
)

func main() {
	log.Println("Starting browser automation with mock site")

	// Load environment variables
	loadDotEnv()

	email := os.Getenv("MOCK_EMAIL")
	password := os.Getenv("MOCK_PASSWORD")
	if email == "" || password == "" {
		email = "test@example.com"
		password = "password123"
		log.Println("Warning: using default credentials")
	}

	// Launch browser
	u := launcher.New().
		Headless(false).
		Leakless(false).
		MustLaunch()

	browser := rod.New().
		ControlURL(u).
		MustConnect()
	defer func() {
		_ = browser.Close()
	}()

	page := browser.MustPage("")
	defer func() {
		_ = page.Close()
	}()

	// Login
	loginURL := "file:///e:/visualstudio/linkedin-automation-poc/mock-site/login.html"
	if err := page.Navigate(loginURL); err != nil {
		log.Fatalf("could not open login page: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		log.Printf("warning: WaitLoad after opening login page: %v", err)
	}

	_ = auth.LoadCookies(page, "data/session.cookie")
	if err := auth.Login(page, email, password); err != nil {
		log.Fatalf("login failed: %v", err)
	}
	_ = auth.SaveCookies(page, "data/session.cookie")

	// Get search configuration
	config := search.DefaultSearchConfig()
	config.PageURL = "file:///e:/visualstudio/linkedin-automation-poc/mock-site/search.html"

	// Navigate to search page
	if err := page.Navigate(config.PageURL); err != nil {
		log.Fatalf("could not open search page: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		log.Printf("warning: WaitLoad after opening search page: %v", err)
	}

	connCfg := connect.ConnectConfig{
		DailyLimit:    5,
		StoragePath:   "data/sent_requests.json",
		PersonalNote:  "", // Can be set to send personalized notes with connect requests
		NoteCharLimit: 300, // LinkedIn character limit for connection notes
	}

	normalize := func(url string) string {
		if url == "" {
			return ""
		}
		if strings.HasPrefix(url, "file://") || strings.HasPrefix(url, "http") {
			return url
		}
		return "file:///e:/visualstudio/linkedin-automation-poc/mock-site/" + url
	}

	// Helper function to process a search flow
	processSearchFlow := func(flowName, query string) {
		log.Printf("\n=== Flow: %s (query=%q) ===", flowName, query)

		// Check if this is a company search
		isCompanySearch := strings.Contains(strings.ToLower(flowName), "company")

		// Step 1: Navigate to search page
		log.Println("Step 1: Navigating to search page...")
		if err := page.Navigate(config.PageURL); err != nil {
			log.Printf("ERROR: could not navigate to search page: %v", err)
			return
		}
		if err := page.WaitLoad(); err != nil {
			log.Printf("warning: WaitLoad after navigating to search page: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Step 2: Enter search query and click search button
		log.Printf("Step 2: Entering search query '%s' and clicking search button...", query)
		searchInput, err := page.Element(config.SearchInputID)
		if err != nil || searchInput == nil {
			log.Printf("ERROR: search input not found: %v", err)
			return
		}
		_ = searchInput.SelectAllText()
		if err := searchInput.Input(query); err != nil {
			log.Printf("ERROR: could not enter search query: %v", err)
			return
		}
		time.Sleep(500 * time.Millisecond)

		// Click search button
		searchBtn, err := page.Element(config.SearchButtonID)
		if err != nil || searchBtn == nil {
			log.Printf("ERROR: search button not found: %v", err)
			return
		}
		if err := searchBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Printf("ERROR: could not click search button: %v", err)
			return
		}
		log.Println("✓ Search button clicked")

		// Wait for search results to appear
		log.Println("Waiting for search results...")
		time.Sleep(2 * time.Second)
		
		// Get search results with pagination info
		searchResults, err := search.SearchFirstPage(page, query, config)
		if err != nil {
			log.Printf("WARNING: could not get search results: %v", err)
		} else if searchResults != nil {
			log.Printf("Search found %d profiles across %d pages", searchResults.TotalProfiles, searchResults.TotalPages)
		}

		var targetURL string
		var targetName string

		if isCompanySearch {
			// For company search, navigate directly to company profile page
			log.Printf("Step 3: Detected company search, navigating to company profile: %s", query)
			targetURL = normalize("company.html?id=" + query)
			targetName = query
			
			if err := page.Navigate(targetURL); err != nil {
				log.Printf("ERROR: could not navigate to company profile: %v", err)
				return
			}
			if err := page.WaitLoad(); err != nil {
				log.Printf("warning: WaitLoad after navigating to company: %v", err)
			}
			time.Sleep(2 * time.Second)
			
			// Verify company page loaded
			companyNameEl, err := page.Element("#company-name")
			if err != nil || companyNameEl == nil {
				log.Printf("ERROR: company page did not load correctly (company-name element not found)")
				return
			}
			companyName, _ := companyNameEl.Text()
			log.Printf("✓ Company page loaded successfully: %s", companyName)
			targetName = companyName
		} else {
			// For person search, find and click on first profile
			log.Println("Step 3: Looking for profile links in search results...")
			profileLinks, err := page.Elements(config.ProfileLinkSel)
			if err != nil || len(profileLinks) == 0 {
				log.Printf("ERROR: no profile links found in search results")
				return
			}

			// Click on the first profile link
			firstProfileLink := profileLinks[0]
			profileName, _ := firstProfileLink.Text()
			log.Printf("Step 4: Clicking on profile: %s", strings.TrimSpace(profileName))
			
			// Get profile URL before clicking
			profileHref, _ := firstProfileLink.Attribute("href")
			if profileHref != nil {
				targetURL = normalize(*profileHref)
			}
			
			if err := firstProfileLink.Click(proto.InputMouseButtonLeft, 1); err != nil {
				log.Printf("ERROR: could not click profile link: %v", err)
				return
			}
			log.Println("✓ Profile link clicked")

			// Step 5: Wait for profile page to load
			log.Println("Step 5: Waiting for profile page to load...")
			if err := page.WaitLoad(); err != nil {
				log.Printf("warning: WaitLoad after clicking profile: %v", err)
			}
			time.Sleep(2 * time.Second)

			// Verify we're on the profile page, navigate directly if needed
			currentURL := ""
			if result, err := page.Eval("() => location.href"); err == nil {
				currentURL = result.Value.Str()
			}
			log.Printf("Current URL: %s", currentURL)
			
			if targetURL != "" && !strings.Contains(currentURL, "profile.html") {
				log.Printf("Not on profile page, navigating directly to: %s", targetURL)
				if err := page.Navigate(targetURL); err != nil {
					log.Printf("ERROR: could not navigate to profile: %v", err)
					return
				}
				if err := page.WaitLoad(); err != nil {
					log.Printf("warning: WaitLoad after navigating to profile: %v", err)
				}
				time.Sleep(1 * time.Second)
			}
			
			// Check if profile page loaded correctly
			nameEl, err := page.Element("#name")
			if err != nil || nameEl == nil {
				log.Printf("ERROR: profile page did not load correctly (name element not found)")
				return
			}
			nameText, _ := nameEl.Text()
			log.Printf("✓ Profile page loaded successfully: %s", nameText)
			targetName = nameText
		}

		// Step 6: Click connect/follow button
		if isCompanySearch {
			log.Println("Step 6: Clicking 'Follow Company' button...")
			connectBtn, err := page.Element("#connect-btn")
			if err != nil || connectBtn == nil {
				log.Printf("ERROR: follow company button not found: %v", err)
				return
			}
			// Scroll to button first
			connectBtn.ScrollIntoView()
			time.Sleep(500 * time.Millisecond)
			if err := connectBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
				log.Printf("ERROR: could not click follow company button: %v", err)
				return
			}
			log.Println("✓ Follow Company button clicked")
			time.Sleep(1 * time.Second)
		} else {
			log.Println("Step 6: Clicking connect button...")
			// We're already on the profile page, so click directly without navigating
			connectBtn, err := page.Element("#connect-btn")
			if err != nil || connectBtn == nil {
				log.Printf("ERROR: connect button not found: %v", err)
				return
			}
			// Scroll to button first
			connectBtn.ScrollIntoView()
			time.Sleep(500 * time.Millisecond)
			
			// Check rate limit before clicking
			if err := ratelimit.CheckAndIncrement("connect", connCfg.DailyLimit, "data/quotas.json"); err != nil {
				log.Printf("WARNING: rate limit reached: %v", err)
				// Continue anyway for testing, but log the warning
			}
			
			if err := connectBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
				log.Printf("ERROR: could not click connect button: %v", err)
				return
			}
			log.Println("✓ Connect button clicked - connection request sent")
			time.Sleep(1 * time.Second)
		}

		// Step 7: Send message
		log.Println("Step 7: Sending message...")
		var msgText string
		var vars map[string]string

		if isCompanySearch {
			msgText = "Hi {{company}}, I'm interested in learning more about opportunities at your company."
			vars = map[string]string{
				"company": targetName,
			}
		} else {
			msgText = "Hi {{first_name}}, thanks for connecting — are there any openings at {{company}}?"
			firstName := strings.Split(targetName, " ")[0]
			vars = map[string]string{
				"first_name": firstName,
				"company":    "your company", // Default placeholder
			}

			// Try to get company from profile
			if companyEl, err := page.Element("#company"); err == nil && companyEl != nil {
				if companyText, err := companyEl.Text(); err == nil {
					vars["company"] = companyText
				}
			}
		}

		// Send message directly (we're already on the page)
		renderedMsg := message.RenderTemplate(msgText, vars)
		
		// Check rate limit
		if err := ratelimit.CheckAndIncrement("message", 5, "data/quotas.json"); err != nil {
			log.Printf("WARNING: message rate limit reached: %v", err)
			// Continue anyway for testing
		}
		
		// Find message box
		msgBox, err := page.Element("#message-box")
		if err != nil || msgBox == nil {
			log.Printf("ERROR: message box not found: %v", err)
			return
		}
		
		// Scroll to message box
		msgBox.ScrollIntoView()
		time.Sleep(500 * time.Millisecond)
		
		// Type message with human-like typing
		log.Printf("Typing message: %s", renderedMsg)
		if err := behavior.HumanType(msgBox, renderedMsg); err != nil {
			log.Printf("ERROR: could not type message: %v", err)
			return
		}
		time.Sleep(500 * time.Millisecond)
		
		// Click send button
		sendBtn, err := page.Element("#send-btn")
		if err != nil || sendBtn == nil {
			log.Printf("ERROR: send button not found: %v", err)
			return
		}
		if err := sendBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Printf("ERROR: could not click send button: %v", err)
			return
		}
		log.Printf("✓ Message sent to %s", targetName)
		time.Sleep(1 * time.Second)

		log.Printf("✓ Flow '%s' completed successfully\n", flowName)
	}

	// Execute all search flows in order
	log.Println("\n========================================")
	log.Println("Starting LinkedIn Automation")
	log.Println("========================================\n")

	processSearchFlow("Search by Name", "Bob")
	processSearchFlow("Search by Company", "VisionaryAI")
	processSearchFlow("Search by Location", "San Francisco")
	processSearchFlow("Search by Position", "Engineer")

	// Process pending messages for newly accepted connections
	log.Println("\n========================================")
	log.Println("Processing Pending Messages for Accepted Connections")
	log.Println("========================================\n")
	
	schedulerCfg := scheduler.SchedulerConfig{
		PendingPath:   "data/pending_messages.json",
		TemplatesPath: "data/templates.json",
		MsgStorage:    "data/sent_messages.json",
	}
	
	if err := scheduler.ProcessPending(page, schedulerCfg); err != nil {
		log.Printf("ERROR: processing pending messages failed: %v", err)
	} else {
		log.Println("✓ Pending messages processed")
	}

	// Post interaction feature - scroll, like, and comment on posts
	log.Println("\n========================================")
	log.Println("Starting Post Interaction Feature")
	log.Println("========================================\n")

	// Navigate back to search page where posts are displayed
	log.Println("Navigating to search page for post interaction...")
	if err := page.Navigate(config.PageURL); err != nil {
		log.Printf("ERROR: could not navigate to search page: %v", err)
	} else {
		if err := page.WaitLoad(); err != nil {
			log.Printf("warning: WaitLoad after navigating: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Interact with posts (scroll, like, comment)
		// Only interact with 2 posts, then scroll down
		if err := post.InteractWithPosts(page, 2); err != nil {
			log.Printf("ERROR: post interaction failed: %v", err)
		}
		
		// Scroll down after interacting with posts
		log.Println("Scrolling down after post interactions...")
		post.HumanScroll(page, 500)
		time.Sleep(1 * time.Second)
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

// waitForConnected polls the #connect-status element until it contains "connected" or timeout elapses.
func waitForConnected(page *rod.Page, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if el, err := page.Element("#connect-status"); err == nil && el != nil {
			if txt, err := el.Text(); err == nil {
				s := strings.ToLower(strings.TrimSpace(txt))
				if strings.Contains(s, "connected") || strings.Contains(s, "accepted") {
					return true
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
