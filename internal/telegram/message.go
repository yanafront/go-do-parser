package telegram

import "strings"

type Post struct {
	SourceChannel  string
	MessageID      int
	Text           string
	HasMedia       bool
	MediaKind      string
	FileID         string
	Caption        string
	GroupedID      int64
	PosterUsername string
	PosterPhone    string
}

type FetchResult struct {
	Posts []Post
	MaxID int
}

func PostBody(post Post) string {
	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	return text
}
