package outreach

import (
	"strings"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

func formatMessage(raw string) (string, []tg.MessageEntityClass) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, nil
	}
	if strings.Contains(raw, "<") {
		raw = strings.ReplaceAll(raw, "\n", "<br>")
		eb := entity.Builder{}
		if err := html.HTML(strings.NewReader(raw), &eb, html.Options{}); err != nil {
			return strings.ReplaceAll(raw, "<br>", "\n"), nil
		}
		text, entities := eb.Complete()
		return text, entities
	}
	return raw, nil
}
