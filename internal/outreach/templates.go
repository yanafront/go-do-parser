package outreach

import (
	"fmt"
	"math/rand"
	"strings"
)

const sourcePlaceholder = "{{source}}"
const sourceNamePlaceholder = "{{source_name}}"

var seekerTemplates = []string{
	`Добрый день!

Увидели ваше объявление: {{source}}

Если сейчас ищете подработку или новую работу, возможно, вам будет полезен сервис Podrabotki.by. Мы собрали вакансии в одном месте, чтобы не приходилось искать их по десяткам Telegram-каналов.

Можно быстро посмотреть предложения, выбрать подходящую смену и сразу связаться с работодателем.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,

	`Здравствуйте!

Нашли ваш контакт в {{source_name}}: {{source}}

Если вы в поиске работы или подработки — загляните на Podrabotki.by. Там собраны актуальные предложения, можно сразу откликнуться работодателю.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Открыть вакансии</a>`,

	`Добрый день!

Пишем по поводу вашего объявления ({{source}}).

На podrabotki.by можно посмотреть свежие смены и вакансии без бесконечного скролла по каналам. Если сейчас ищете работу — сервис может сэкономить время.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,
}

func HasSeekerTemplates() bool {
	for _, t := range seekerTemplates {
		if strings.TrimSpace(t) != "" {
			return true
		}
	}
	return false
}

func PickSeekerMessage(sourceLink, sourceName string) string {
	templates := make([]string, 0, len(seekerTemplates)+1)
	for _, t := range seekerTemplates {
		if strings.TrimSpace(t) != "" {
			templates = append(templates, t)
		}
	}
	if len(templates) == 0 {
		return ""
	}
	tpl := templates[rand.Intn(len(templates))]
	return FillSourcePlaceholders(tpl, sourceLink, sourceName)
}

func FillSourcePlaceholders(tpl, sourceLink, sourceName string) string {
	link := strings.TrimSpace(sourceLink)
	name := strings.TrimSpace(sourceName)
	if name == "" {
		name = "источник объявления"
	}
	if link == "" {
		link = name
	}
	out := strings.ReplaceAll(tpl, sourcePlaceholder, link)
	out = strings.ReplaceAll(out, sourceNamePlaceholder, name)
	return out
}

func FormatSourceName(sourceChannel string) string {
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
	if strings.Contains(sourceChannel, "onliner") {
		return "барахолка Onliner"
	}
	return "@" + strings.TrimPrefix(sourceChannel, "@")
}

func FormatSourceLink(sourceChannel, sourceLink string, messageID int) string {
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
