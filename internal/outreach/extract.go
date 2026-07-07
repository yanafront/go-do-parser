package outreach

import (
	"regexp"
	"strings"
)

var phoneRE = regexp.MustCompile(`(?i)(?:\+?\s*375[\s\-]*\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|8[\s\-]*0\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|\+?\s*80\d{9})`)
var usernameRE = regexp.MustCompile(`(?i)@([a-zA-Z][a-zA-Z0-9_]{4,31})`)

type Target struct {
	Key  string
	Type string
	Raw  string
}

func ExtractTargets(text string) []Target {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	seen := make(map[string]bool)
	var out []Target

	for _, m := range phoneRE.FindAllString(text, -1) {
		phone := normalizePhone(m)
		if !isBelarusPhone(phone) {
			continue
		}
		key := "p:" + phone
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Target{Key: key, Type: "phone", Raw: phone})
	}

	return out
}

func ExtractAdContacts(text string, skip map[string]bool) (username, phone string) {
	if u, ok := ExtractUsername(text, skip); ok {
		username = u.Raw
	}
	for _, m := range phoneRE.FindAllString(text, -1) {
		p := normalizePhone(m)
		if isBelarusPhone(p) {
			phone = p
			break
		}
	}
	return username, phone
}

func SeekerAdContacts(body, posterUsername, posterPhone string, skip map[string]bool) (adUsername, adPhone string) {
	adUsername, adPhone = ExtractAdContacts(body, skip)
	if adUsername != "" || adPhone != "" {
		return adUsername, adPhone
	}
	if posterUsername != "" && isTelegramUsername(posterUsername) {
		if u, ok := ExtractUsername("@"+strings.TrimPrefix(posterUsername, "@"), skip); ok {
			return u.Raw, ""
		}
	}
	if posterPhone != "" {
		p := normalizePhone(posterPhone)
		if isBelarusPhone(p) {
			return "", p
		}
	}
	return "", ""
}

func SeekerTarget(text, posterUsername, posterPhone string, skip map[string]bool) (Target, bool) {
	if t, ok := ExtractSeekerTarget(text, skip); ok {
		return t, true
	}
	if posterUsername != "" && isTelegramUsername(posterUsername) {
		if t, ok := ExtractUsername("@"+strings.TrimPrefix(posterUsername, "@"), skip); ok {
			return t, true
		}
	}
	if posterPhone != "" {
		p := normalizePhone(posterPhone)
		if isBelarusPhone(p) {
			return Target{Key: "p:" + p, Type: "phone", Raw: p}, true
		}
	}
	return Target{}, false
}

func isTelegramUsername(s string) bool {
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	if len(s) < 5 || len(s) > 32 {
		return false
	}
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func ExtractSeekerContacts(text string, skip map[string]bool) (poster, adUser, adPhone string) {
	seen := make(map[string]bool)
	var usernames []string
	for _, m := range usernameRE.FindAllStringSubmatch(text, -1) {
		u := strings.ToLower(m[1])
		if skip[u] || strings.HasSuffix(u, "bot") || seen[u] {
			continue
		}
		seen[u] = true
		usernames = append(usernames, u)
	}
	if len(usernames) > 0 {
		poster = usernames[0]
	}
	if len(usernames) > 1 {
		adUser = usernames[1]
	}
	for _, m := range phoneRE.FindAllString(text, -1) {
		p := normalizePhone(m)
		if isBelarusPhone(p) {
			adPhone = p
			break
		}
	}
	return poster, adUser, adPhone
}

func ExtractSeekerTarget(text string, skip map[string]bool) (Target, bool) {
	if u, ok := ExtractUsername(text, skip); ok {
		return u, true
	}
	for _, t := range ExtractTargets(text) {
		return t, true
	}
	return Target{}, false
}

func ExtractUsername(text string, skip map[string]bool) (Target, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Target{}, false
	}
	for _, m := range usernameRE.FindAllStringSubmatch(text, -1) {
		u := strings.ToLower(m[1])
		if skip[u] {
			continue
		}
		if strings.HasSuffix(u, "bot") {
			continue
		}
		return Target{Key: "u:" + u, Type: "username", Raw: u}, true
	}
	return Target{}, false
}

func isBelarusPhone(phone string) bool {
	return strings.HasPrefix(phone, "+375") && len(phone) >= 13
}

func normalizePhone(s string) string {
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(s))
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

func BuildSkipList(sources []string, extra ...string) map[string]bool {
	skip := make(map[string]bool)
	add := func(s string) {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "@")
		s = strings.TrimPrefix(s, "https://t.me/")
		s = strings.TrimPrefix(s, "t.me/")
		if s != "" {
			skip[strings.ToLower(s)] = true
		}
	}
	for _, s := range sources {
		add(s)
	}
	for _, s := range extra {
		add(s)
	}
	return skip
}
