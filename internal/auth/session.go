package auth

import (
	"io/ioutil"
	"log"
	"strings"

	"github.com/go-rod/rod"
)

// SaveCookies saves document.cookie string to a local file.
// This is a simple persistence mechanism for the mock site.
func SaveCookies(page *rod.Page, path string) error {
	if page == nil {
		return nil
	}
	// Use MustEval to retrieve document.cookie as a string
	val := page.MustEval("() => document.cookie")
	cookieStr := val.String()
	if err := ioutil.WriteFile(path, []byte(cookieStr), 0o644); err != nil {
		return err
	}
	log.Printf("saved cookies to %s", path)
	return nil
}

// LoadCookies reads cookie string from file and sets document.cookie entries on the page.
// It does not attempt to validate domains; caller should navigate to the appropriate page first.
func LoadCookies(page *rod.Page, path string) error {
	if page == nil {
		return nil
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	cookieStr := string(b)
	cookieStr = strings.TrimSpace(cookieStr)
	if cookieStr == "" {
		return nil
	}
	parts := strings.Split(cookieStr, "; ")
	for _, p := range parts {
		// set each cookie via document.cookie
		js := `document.cookie = "` + p + `; path=/";`
		page.MustEval(js)
	}
	log.Printf("loaded cookies from %s", path)
	return nil
}
