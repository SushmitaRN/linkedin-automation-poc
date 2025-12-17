package auth

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
)

// Login performs login on the provided page using given credentials.
// The caller is responsible for navigating the page to the mock login URL first.
// Returns an error on failure; avoids panics so callers can handle flow.
func Login(page *rod.Page, email, password string) error {
	if page == nil {
		return errors.New("page is nil")
	}
	if email == "" || password == "" {
		return errors.New("email and password must be provided")
	}

	log.Println("Entering credentials (mock site)")

	// Email
	emailEl, err := page.Element("#email")
	if err != nil || emailEl == nil {
		return errors.New("email input not found on login page")
	}
	if err := behavior.HumanType(emailEl, email); err != nil {
		return err
	}

	// Small human-like pause
	behavior.SleepHuman(150*time.Millisecond, 350*time.Millisecond)

	// Password
	passEl, err := page.Element("#password")
	if err != nil || passEl == nil {
		return errors.New("password input not found on login page")
	}
	if err := behavior.HumanType(passEl, password); err != nil {
		return err
	}

	log.Println("Submitting login form")
	loginBtn, err := page.Element("#login-btn")
	if err != nil || loginBtn == nil {
		return errors.New("login button not found on login page")
	}
	if err := loginBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	// Wait for status message or redirect to search page (polling)
	log.Println("Waiting for login response...")
	var lastStatus string
	for i := 0; i < 20; i++ {
		// check status text
		if statusEl, _ := page.Element("#status"); statusEl != nil {
			txt, _ := statusEl.Text()
			lastStatus = txt
			if strings.Contains(strings.ToLower(txt), "success") || strings.Contains(strings.ToLower(txt), "login successful") {
				log.Println("Login successful (mock)")
				// allow redirect to happen
				time.Sleep(500 * time.Millisecond)
				return nil
			}
		}
		// check url for redirect
		// Check location.href via MustEval and string conversion
		u := page.MustEval(`() => location.href`).Str()
		if u != "" && strings.Contains(u, "search.html") {
			log.Println("Redirect detected to search.html")
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	return errors.New("login failed: " + lastStatus)
}
