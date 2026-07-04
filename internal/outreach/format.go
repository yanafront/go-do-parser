package outreach

import (
	"strings"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

func formatMessage(raw string) (string, []tg.MessageEntityClass) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.Contains(raw, "<") {
		return raw, nil
	}
	eb := entity.Builder{}
	if err := html.HTML(strings.NewReader(raw), &eb, html.Options{}); err != nil {
		return raw, nil
	}
	text, entities := eb.Complete()
	return text, entities
}
