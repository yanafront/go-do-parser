package telegram

import (
	"regexp"
	"strings"
)

var belarusPhoneRE = regexp.MustCompile(`(?i)(?:\+?\s*375[\s\-]*\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|8[\s\-]*0\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|\+?\s*80\d{9})`)

func HasContact(post Post) bool {
	text := postText(post)
	if text == "" {
		return false
	}
	if strings.Contains(text, "@") {
		return true
	}
	if strings.Contains(text, "+") {
		return true
	}
	if strings.Contains(text, "375") {
		return true
	}
	if belarusPhoneRE.MatchString(text) {
		return true
	}
	if strings.Contains(text, "8-0") || strings.Contains(text, "8 0") {
		return true
	}
	return false
}

func IsBlocked(post Post) bool {
	text := strings.ToLower(postText(post))
	return strings.Contains(text, "заблокирован") || strings.Contains(text, "внимание")
}

func postText(post Post) string {
	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	return text
}
