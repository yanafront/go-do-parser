package post

import "strings"

type Post struct {
	SourceChannel string
	MessageID     int
	Text          string
	HasMedia      bool
	MediaKind     string
	Caption       string
	GroupedID     int64
}

type FetchResult struct {
	Posts []Post
	MaxID int
}

func PlainText(p Post) string {
	text := strings.TrimSpace(p.Text)
	if text == "" {
		text = strings.TrimSpace(p.Caption)
	}
	return text
}
