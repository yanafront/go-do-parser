package telegram

import (
	"context"
	"fmt"
	"strings"
)

func MessageLink(channelUsername string, channelID int64, messageID int) string {
	if messageID <= 0 {
		return ""
	}
	username := normalizeUsername(channelUsername)
	if username != "" && channelHasPublicUsername(username) {
		return fmt.Sprintf("https://t.me/%s/%d", username, messageID)
	}
	if channelID > 0 {
		return fmt.Sprintf("https://t.me/c/%d/%d", channelID, messageID)
	}
	if username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", username, messageID)
	}
	return ""
}

func channelHasPublicUsername(username string) bool {
	if username == "" {
		return false
	}
	if strings.HasPrefix(username, "-") {
		return false
	}
	for _, c := range username {
		if c < '0' || c > '9' {
			return true
		}
	}
	return false
}

func (r *Reader) MessageLink(ctx context.Context, channelUsername string, messageID int) string {
	if !r.ready || r.api == nil || r.channels == nil {
		return MessageLink(channelUsername, 0, messageID)
	}
	username := normalizeUsername(channelUsername)
	if username == "" {
		return ""
	}
	channel, err := r.channels.get(ctx, username)
	if err != nil {
		return MessageLink(channelUsername, 0, messageID)
	}
	linkUsername := channel.Username
	if linkUsername == "" {
		linkUsername = username
	}
	return MessageLink(linkUsername, channel.ID, messageID)
}
