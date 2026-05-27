package telegram

import "strings"

func HasContact(post Post) bool {
	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	if text == "" {
		return false
	}
	if strings.Contains(text, "+") {
		return true
	}
	if strings.Contains(text, "375") {
		return true
	}
	if strings.Contains(text, "80") {
		return true
	}
	if strings.Contains(text, "@") {
		return true
	}
	return false
}
