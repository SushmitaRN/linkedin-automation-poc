package post

import (
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sushmitaRN/linkedin-automation-poc/internal/behavior"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ScrollToElement smoothly scrolls to an element with human-like behavior
func ScrollToElement(page *rod.Page, element *rod.Element) error {
	if page == nil || element == nil {
		return nil
	}

	// Get element position using JavaScript
	result, err := page.Eval(`(el) => {
		const rect = el.getBoundingClientRect();
		const currentScroll = window.pageYOffset || document.documentElement.scrollTop;
		return {
			targetY: currentScroll + rect.top - 100,
			currentY: currentScroll
		};
	}`, element)
	
	if err != nil {
		// Fallback: simple scroll into view
		element.ScrollIntoView()
		behavior.ReadingPause()
		return nil
	}

	// Extract values from result
	obj := result.Value.Map()
	currentY := int(obj["currentY"].Int())
	targetY := int(obj["targetY"].Int())

	// Smooth scroll in steps (human-like)
	steps := 10 + rand.Intn(10) // 10-20 steps
	if steps == 0 {
		steps = 10
	}
	stepSize := (targetY - currentY) / steps
	if stepSize == 0 {
		stepSize = 1
	}

	for i := 0; i < steps; i++ {
		scrollY := currentY + (stepSize * i)
		page.Eval(`(y) => { window.scrollTo(0, y); }`, scrollY)
		behavior.SleepHuman(50*time.Millisecond, 150*time.Millisecond)
	}

	// Final scroll to exact position
	page.Eval(`(y) => { window.scrollTo(0, y); }`, targetY)
	behavior.ReadingPause()

	return nil
}

// HumanScroll scrolls the page like a human would - gradually and with pauses
func HumanScroll(page *rod.Page, scrollAmount int) {
	if page == nil {
		return
	}

	// Scroll in smaller increments with pauses
	steps := 3 + rand.Intn(5) // 3-7 steps
	stepSize := scrollAmount / steps

	for i := 0; i < steps; i++ {
		page.Eval(`(amount) => { window.scrollBy(0, amount); }`, stepSize)
		behavior.SleepHuman(200*time.Millisecond, 500*time.Millisecond)
	}

	// Small random pause after scrolling
	behavior.ReadingPause()
}

// LikePost likes a post by clicking the like button
func LikePost(page *rod.Page, postElement *rod.Element) error {
	if postElement == nil {
		return nil
	}

	// Find the like button within this post - try multiple selectors
	var likeBtn *rod.Element
	var err error
	
	// Try .like-btn first
	likeBtn, err = postElement.Element(".like-btn")
	if err != nil || likeBtn == nil {
		// Try finding button with onclick containing toggleLike
		allButtons, _ := postElement.Elements("button")
		for _, btn := range allButtons {
			onclick, _ := btn.Attribute("onclick")
			if onclick != nil && strings.Contains(*onclick, "toggleLike") {
				likeBtn = btn
				break
			}
		}
	}

	if likeBtn == nil {
		log.Printf("Like button not found for post")
		return nil
	}

	// Scroll to the post first
	postElement.ScrollIntoView()
	time.Sleep(500 * time.Millisecond)

	// Simulate reading the post before liking
	behavior.ReadingPause()

	// Check if already liked by checking class
	hasLiked, _ := likeBtn.Attribute("class")
	isLiked := false
	if hasLiked != nil {
		isLiked = strings.Contains(*hasLiked, "liked")
	}

	if !isLiked {
		log.Println("Liking post...")
		// Scroll to button
		likeBtn.ScrollIntoView()
		time.Sleep(300 * time.Millisecond)
		
		if err := likeBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Printf("Error clicking like button: %v", err)
			return err
		}
		log.Println("✓ Post liked")
		time.Sleep(500 * time.Millisecond)
	} else {
		log.Println("Post already liked, skipping")
	}

	return nil
}

// CommentOnPost adds a comment to a post
func CommentOnPost(page *rod.Page, postElement *rod.Element, commentText string) error {
	if postElement == nil {
		return nil
	}

	// Get post ID from data attribute
	postID, _ := postElement.Attribute("data-post-id")
	if postID == nil {
		log.Println("Could not find post ID")
		return nil
	}

	log.Printf("Commenting on post ID: %s", *postID)

	// Scroll to post first
	postElement.ScrollIntoView()
	time.Sleep(500 * time.Millisecond)

	// Find and click the comment button to expand comments section
	// Try multiple ways to find the comment toggle button
	var commentToggleBtn *rod.Element
	
	// Method 1: Find button with onclick containing toggleComments
	allButtons, _ := postElement.Elements("button")
	for _, btn := range allButtons {
		onclick, _ := btn.Attribute("onclick")
		if onclick != nil && strings.Contains(*onclick, "toggleComments") {
			commentToggleBtn = btn
			break
		}
	}
	
	// Method 2: Find by text content (contains 💬)
	if commentToggleBtn == nil {
		allActions, _ := postElement.Elements(".post-action")
		for _, action := range allActions {
			text, _ := action.Text()
			if strings.Contains(text, "💬") {
				commentToggleBtn = action
				break
			}
		}
	}

	if commentToggleBtn != nil {
		log.Println("Found comment toggle button, clicking to expand...")
		commentToggleBtn.ScrollIntoView()
		time.Sleep(300 * time.Millisecond)
		
		// Click to expand comments section
		if err := commentToggleBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Printf("Could not click comment toggle: %v", err)
		} else {
			log.Println("✓ Expanded comments section")
			time.Sleep(1 * time.Second) // Wait for comment section to appear
		}
	} else {
		log.Println("Comment toggle button not found, trying to find comment input directly...")
	}

	// Wait a bit for comment section to appear/be visible
	time.Sleep(1 * time.Second)

	// Find comment input - try multiple selectors
	var commentInput *rod.Element
	commentInput, err := page.Element("#comment-input-" + *postID)
	if err != nil || commentInput == nil {
		// Try finding within post element
		commentSection, _ := postElement.Element("#comments-" + *postID)
		if commentSection != nil {
			commentInput, _ = commentSection.Element("input")
		}
	}

	if commentInput == nil {
		log.Printf("Comment input not found for post ID %s", *postID)
		return nil
	}

	log.Println("Found comment input, scrolling to it...")
	commentInput.ScrollIntoView()
	time.Sleep(500 * time.Millisecond)

	// Type comment
	log.Printf("Typing comment: %s", commentText)
	if err := behavior.HumanType(commentInput, commentText); err != nil {
		log.Printf("Error typing comment: %v", err)
		return err
	}

	time.Sleep(500 * time.Millisecond)

	// Find and click post button
	// Try multiple ways to find the post button
	var postBtn *rod.Element
	
	// Method 1: Find button with onclick containing addComment
	allPageButtons, _ := page.Elements("button")
	for _, btn := range allPageButtons {
		onclick, _ := btn.Attribute("onclick")
		if onclick != nil && strings.Contains(*onclick, "addComment("+*postID+")") {
			postBtn = btn
			break
		}
	}
	
	// Method 2: Find button in comment section
	if postBtn == nil {
		commentSection, _ := postElement.Element("#comments-" + *postID)
		if commentSection != nil {
			commentInputDiv, _ := commentSection.Element(".comment-input")
			if commentInputDiv != nil {
				postBtn, _ = commentInputDiv.Element("button")
			}
		}
	}

	if postBtn != nil {
		log.Println("Found post button, clicking...")
		postBtn.ScrollIntoView()
		time.Sleep(300 * time.Millisecond)
		
		if err := postBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Printf("Error clicking post button: %v", err)
			return err
		}
		log.Println("✓ Comment posted successfully")
		time.Sleep(1 * time.Second)
	} else {
		log.Println("Could not find post button for comment")
		return nil
	}

	return nil
}

// InteractWithPosts scrolls through posts, likes some, and comments on some
func InteractWithPosts(page *rod.Page, maxPosts int) error {
	log.Println("\n=== Starting Post Interaction ===")

	// Find all posts
	posts, err := page.Elements(".post")
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		log.Println("No posts found on page")
		return nil
	}

	log.Printf("Found %d posts, will interact with up to %d", len(posts), maxPosts)

	// Limit the number of posts to interact with
	if maxPosts > len(posts) {
		maxPosts = len(posts)
	}

	// Random comments to use
	comments := []string{
		"Great insights!",
		"Thanks for sharing this.",
		"Very informative post.",
		"This is helpful, thank you!",
		"Interesting perspective!",
		"Appreciate the update.",
		"Good to know!",
		"Thanks for the information.",
	}

	for i := 0; i < maxPosts; i++ {
		post := posts[i]
		log.Printf("\n--- Interacting with post %d/%d ---", i+1, maxPosts)

		// Scroll to post
		if err := ScrollToElement(page, post); err != nil {
			log.Printf("Could not scroll to post: %v", err)
			continue
		}

		// Read the post (simulate reading time)
		behavior.ReadingPause()
		behavior.ReadingPause()

		// Always like the post
		log.Println("Attempting to like post...")
		if err := LikePost(page, post); err != nil {
			log.Printf("Error liking post: %v", err)
		}

		// Always comment on the post
		commentText := comments[rand.Intn(len(comments))]
		log.Println("Attempting to comment on post...")
		if err := CommentOnPost(page, post, commentText); err != nil {
			log.Printf("Error commenting on post: %v", err)
		}

		// Scroll down a bit before next post
		if i < maxPosts-1 {
			HumanScroll(page, 200+rand.Intn(300))
		}
	}

	log.Println("\n=== Post Interaction Complete ===")
	return nil
}


