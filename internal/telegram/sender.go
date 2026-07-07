package telegram

import (
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
)

func extractUsers(box tg.MessagesMessagesClass) map[int64]*tg.User {
	users := make(map[int64]*tg.User)
	switch v := box.(type) {
	case *tg.MessagesMessages:
		addUsers(users, v.Users)
	case *tg.MessagesMessagesSlice:
		addUsers(users, v.Users)
	case *tg.MessagesChannelMessages:
		addUsers(users, v.Users)
	}
	return users
}

func addUsers(dst map[int64]*tg.User, items []tg.UserClass) {
	for _, item := range items {
		u, ok := item.(*tg.User)
		if !ok || u == nil {
			continue
		}
		dst[u.ID] = u
	}
}

func messagePosterContact(msg *tg.Message, users map[int64]*tg.User) (username, phone string) {
	if msg == nil {
		return "", ""
	}
	if username, phone, ok := peerUserContact(msg.GetFromID, users); ok {
		return username, phone
	}
	if fwd, ok := msg.GetFwdFrom(); ok {
		if username, phone, ok := peerUserContact(func() (tg.PeerClass, bool) { return fwd.GetFromID() }, users); ok {
			return username, phone
		}
		if name, ok := fwd.GetFromName(); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name), ""
		}
		if author, ok := fwd.GetPostAuthor(); ok && strings.TrimSpace(author) != "" {
			return strings.TrimSpace(author), ""
		}
	}
	if author, ok := msg.GetPostAuthor(); ok && strings.TrimSpace(author) != "" {
		return strings.TrimSpace(author), ""
	}
	if username, phone := contactsFromEntities(msg, users); username != "" || phone != "" {
		return username, phone
	}
	return "", ""
}

func contactsFromEntities(msg *tg.Message, users map[int64]*tg.User) (username, phone string) {
	if msg == nil || len(msg.Entities) == 0 {
		return "", ""
	}
	runes := utf16.Encode([]rune(msg.Message))
	for _, ent := range msg.Entities {
		switch e := ent.(type) {
		case *tg.MessageEntityMentionName:
			if u := users[e.UserID]; u != nil {
				if u.Username != "" {
					return strings.ToLower(u.Username), ""
				}
				if u.Phone != "" {
					return "", u.Phone
				}
			}
		case *tg.MessageEntityMention:
			mention := entityText(runes, e.Offset, e.Length)
			mention = strings.TrimPrefix(strings.TrimSpace(mention), "@")
			if isTelegramUsername(mention) {
				return strings.ToLower(mention), ""
			}
		case *tg.MessageEntityPhone:
			if p := normalizeEntityPhone(entityText(runes, e.Offset, e.Length)); p != "" {
				return "", p
			}
		case *tg.MessageEntityURL:
			if u := usernameFromTMeLink(entityText(runes, e.Offset, e.Length)); u != "" {
				return u, ""
			}
		case *tg.MessageEntityTextURL:
			if u := usernameFromTMeLink(entityText(runes, e.Offset, e.Length)); u != "" {
				return u, ""
			}
		}
	}
	return "", ""
}

func entityText(runes []uint16, offset, length int) string {
	if offset < 0 || length < 0 || offset+length > len(runes) {
		return ""
	}
	return string(utf16.Decode(runes[offset : offset+length]))
}

func usernameFromTMeLink(link string) string {
	link = strings.TrimSpace(strings.ToLower(link))
	link = strings.TrimPrefix(link, "https://")
	link = strings.TrimPrefix(link, "http://")
	link = strings.TrimPrefix(link, "t.me/")
	if strings.HasPrefix(link, "+") || strings.HasPrefix(link, "c/") {
		return ""
	}
	if i := strings.IndexByte(link, '/'); i >= 0 {
		link = link[:i]
	}
	if i := strings.IndexByte(link, '?'); i >= 0 {
		link = link[:i]
	}
	if isTelegramUsername(link) {
		return link
	}
	return ""
}

func normalizeEntityPhone(s string) string {
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(s))
	if strings.HasPrefix(s, "+375") {
		return s
	}
	if strings.HasPrefix(s, "375") {
		return "+" + s
	}
	if strings.HasPrefix(s, "80") && len(s) >= 11 {
		return "+375" + s[2:]
	}
	return ""
}

func peerUserContact(getPeer func() (tg.PeerClass, bool), users map[int64]*tg.User) (username, phone string, ok bool) {
	peer, hasPeer := getPeer()
	if !hasPeer {
		return "", "", false
	}
	peerUser, isUser := peer.(*tg.PeerUser)
	if !isUser {
		return "", "", false
	}
	u := users[peerUser.UserID]
	if u == nil {
		return "", "", false
	}
	if u.Username != "" {
		return strings.ToLower(u.Username), "", true
	}
	if u.Phone != "" {
		return "", u.Phone, true
	}
	return "", "", false
}

func isTelegramUsername(s string) bool {
	if s == "" {
		return false
	}
	if len(s) < 5 || len(s) > 32 {
		return false
	}
	if s[0] < 'a' && s[0] > 'z' && s[0] < 'A' && s[0] > 'Z' {
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
