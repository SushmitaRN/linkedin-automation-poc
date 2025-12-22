package auth

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func Login(page *rod.Page, email, password string) error {
	if page == nil {
		return errors.New("page is nil")
	}

	log.Println("Logging in (mock site)")

	page.MustElement("#email").MustInput(email)
	page.MustElement("#password").MustInput(password)
	page.MustElement("#login-btn").MustClick()

	// Wait for redirect to search.html
	timeout := time.After(5 * time.Second)
	tick := time.Tick(200 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return errors.New("login timeout")
		case <-tick:
			href := page.MustEval(`() => location.href`).String()
			if strings.Contains(href, "search.html") {
				log.Println("✓ Login successful")
				return nil
			}
		}
	}
}
