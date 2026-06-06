package telegram

import "strings"

func NormalizePhone(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "+") {
		return s
	}
	if strings.HasPrefix(s, "375") {
		return "+" + s
	}
	if strings.HasPrefix(s, "80") && len(s) >= 10 {
		return "+375" + s[2:]
	}
	if strings.HasPrefix(s, "0") {
		return "+375" + strings.TrimPrefix(s, "0")
	}
	return "+" + s
}

func MaskPhone(s string) string {
	if len(s) <= 6 {
		return s
	}
	return s[:4] + strings.Repeat("*", len(s)-7) + s[len(s)-3:]
}
