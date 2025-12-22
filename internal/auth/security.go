package auth

import (
	"errors"
	"log"
	"strings"

	"github.com/go-rod/rod"
)

// DetectSecurityCheckpoints checks for 2FA, captcha, or other security challenges
func DetectSecurityCheckpoints(page *rod.Page) error {
	if page == nil {
		return nil
	}

	// Check for 2FA challenge
	if twoFA, err := page.Element("#two-factor-input, #verification-code, .two-factor"); err == nil && twoFA != nil {
		log.Println("⚠️ 2FA challenge detected - manual intervention required")
		return errors.New("2FA challenge detected - please complete manually")
	}

	// Check for CAPTCHA
	if captcha, err := page.Element("#captcha, .g-recaptcha, .captcha-container, [data-callback]"); err == nil && captcha != nil {
		log.Println("⚠️ CAPTCHA detected - manual intervention required")
		return errors.New("CAPTCHA detected - please complete manually")
	}

	// Check for security challenge text
	result, err := page.Eval(`() => document.body.innerText`)
	if err == nil {
		bodyText := result.Value.Str()
		bodyLower := strings.ToLower(bodyText)
		if strings.Contains(bodyLower, "verify your identity") ||
			strings.Contains(bodyLower, "security challenge") ||
			strings.Contains(bodyLower, "verify it's you") {
			log.Println("⚠️ Security challenge detected - manual intervention may be required")
			return errors.New("security challenge detected")
		}
	}

	return nil
}

