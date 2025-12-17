package behavior

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// SleepHuman sleeps for a random duration between min and max.
// If min >= max, sleeps for min.
func SleepHuman(min, max time.Duration) {
	if min <= 0 && max <= 0 {
		time.Sleep(800 * time.Millisecond)
		return
	}
	if min >= max {
		time.Sleep(min)
		return
	}
	delta := max - min
	time.Sleep(min + time.Duration(rand.Int63n(int64(delta))))
}

// HumanType simulates human typing into the provided element.
// Sets input value directly via JavaScript (more reliable than Input)
// and simulates reading/thinking time.
func HumanType(el *rod.Element, text string) error {
	if el == nil {
		return nil
	}
	// click to focus
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	// Use Element.Input to reliably simulate typing
	if err := el.Input(text); err != nil {
		// fallback: set value via JS if Input fails
		page := el.Page()
		_, _ = page.Eval(`(el, txt) => { el.value = txt; }`, el, text)
	}

	// Simulate reading/thinking time with inter-character delays
	for range text {
		SleepHuman(80*time.Millisecond, 220*time.Millisecond)
	}

	return nil
}

// RandomScroll performs a small randomized scroll on the page to simulate reading.
func RandomScroll(page *rod.Page) {
	if page == nil {
		return
	}
	// small scroll by random offset
	offset := rand.Intn(300) - 150 // -150 .. +150
	js := "window.scrollBy(0, " + strconv.Itoa(offset) + ");"
	page.Eval(js)
}

// ThinkPause adds a longer human-like thinking pause (useful between actions).
func ThinkPause() {
	SleepHuman(1500*time.Millisecond, 3000*time.Millisecond)
}

// ReadingPause adds a pause as if reading content on the page.
func ReadingPause() {
	SleepHuman(800*time.Millisecond, 2000*time.Millisecond)
}
