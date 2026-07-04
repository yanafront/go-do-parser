package outreach

import (
	"regexp"
	"strings"
)

var (
	usernameRE = regexp.MustCompile(`(?i)@([a-zA-Z][a-zA-Z0-9_]{4,31})`)
	phoneRE    = regexp.MustCompile(`(?i)(?:\+?\s*375[\s\-]*\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|8[\s\-]*0\d{2}[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}|\+?\s*80\d{9})`)
)

type Target struct {
	Key  string
	Type string
	Raw  string
}

func ExtractTargets(text string, skipUsernames map[string]bool) []Target {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	seen := make(map[string]bool)
	var out []Target

	for _, m := range usernameRE.FindAllStringSubmatch(text, -1) {
		u := strings.ToLower(m[1])
		if skipUsernames[u] {
			continue
		}
		if strings.HasSuffix(u, "bot") {
			continue
		}
		key := "u:" + u
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Target{Key: key, Type: "username", Raw: u})
	}

	for _, m := range phoneRE.FindAllString(text, -1) {
		phone := normalizePhone(m)
		if phone == "" {
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
