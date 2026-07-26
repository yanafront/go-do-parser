package seekermsg

import (
	"fmt"
	"os"
	"strings"
)

const (
	sourcePlaceholder     = "{{source}}"
	sourceNamePlaceholder = "{{source_name}}"
)

var exampleTemplate = `Добрый день! Увидели ваше объявление в {{source_name}}. Если ищете работу — заходите к нам: <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`

func Preview(sourceChannel, sourceLink string, messageID int) string {
	link := formatSourceLink(sourceChannel, sourceLink, messageID)
	name := formatSourceName(sourceChannel)
	if custom := strings.TrimSpace(os.Getenv("SEEKER_MESSAGE")); custom != "" {
		custom = strings.NewReplacer("\\n", "\n", "\\t", "\t").Replace(custom)
		if strings.Contains(custom, sourcePlaceholder) || strings.Contains(custom, sourceNamePlaceholder) {
			return fill(custom, link, name)
		}
		if link != "" {
			return custom + "\n\nИсточник: " + link
		}
		return custom
	}
	return fill(exampleTemplate, link, name)
}

func fill(tpl, sourceLink, sourceName string) string {
	link := strings.TrimSpace(sourceLink)
	name := strings.TrimSpace(sourceName)
	if name == "" {
		name = "источник объявления"
	}
	if link == "" {
		link = name
	}
	out := strings.ReplaceAll(tpl, sourcePlaceholder, link)
	return strings.ReplaceAll(out, sourceNamePlaceholder, name)
}

func formatSourceName(sourceChannel string) string {
	sourceChannel = strings.TrimSpace(sourceChannel)
	if sourceChannel == "" {
		return "источник объявления"
	}
	if strings.HasPrefix(sourceChannel, "onliner:") {
		return "барахолка Onliner"
	}
	if strings.HasPrefix(sourceChannel, "@") {
		return sourceChannel
	}
	return "@" + strings.TrimPrefix(sourceChannel, "@")
}

func formatSourceLink(sourceChannel, sourceLink string, messageID int) string {
	link := strings.TrimSpace(sourceLink)
	if link != "" {
		return link
	}
	sourceChannel = strings.TrimSpace(sourceChannel)
	if strings.HasPrefix(sourceChannel, "onliner:") {
		if messageID > 0 {
			return fmt.Sprintf("https://baraholka.onliner.by/viewtopic.php?t=%d", messageID)
		}
		return "https://baraholka.onliner.by"
	}
	username := strings.TrimPrefix(sourceChannel, "@")
	if username != "" && messageID > 0 {
		return fmt.Sprintf("https://t.me/%s/%d", username, messageID)
	}
	if username != "" {
		return "https://t.me/" + username
	}
	return ""
}
