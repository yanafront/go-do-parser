package onliner

import (
	"regexp"
	"strings"

	"github.com/anadubesko/go-do-parser/internal/outreach"
)

var emailRE = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)

type Contacts struct {
	Phone    string
	Email    string
	Telegram string
}

func ExtractContacts(text string) Contacts {
	text = strings.TrimSpace(text)
	if text == "" {
		return Contacts{}
	}

	skip := outreach.BuildSkipList(nil)
	telegram, phone := outreach.ExtractAdContacts(text, skip)

	if phone == "" {
		for _, t := range outreach.ExtractTargets(text) {
			if t.Type == "phone" {
				phone = t.Raw
				break
			}
		}
	}

	if telegram == "" {
		if t, ok := outreach.ExtractUsername(text, skip); ok {
			telegram = t.Raw
		}
	}

	email := ""
	if m := emailRE.FindString(text); m != "" {
		email = strings.ToLower(m)
	}

	return Contacts{
		Phone:    phone,
		Email:    email,
		Telegram: telegram,
	}
}
