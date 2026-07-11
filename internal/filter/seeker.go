package filter

import "strings"

func IsJobSeekerText(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(text, "ищу подработку") || strings.Contains(text, "ищу работу")
}
