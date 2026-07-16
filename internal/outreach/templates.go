package outreach

import (
	"fmt"
	"math/rand"
	"strings"
)

const sourcePlaceholder = "{{source}}"
const sourceNamePlaceholder = "{{source_name}}"

var seekerTemplates = []string{
	`Добрый день! Увидели ваше объявление в {{source_name}}. Если ищете работу — заходите к нам: <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,

	`Здравствуйте! Увидели ваше объявление в {{source_name}} ({{source}}). Если сейчас в поиске работы — будем рады видеть вас на <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,

	`Добрый день!

Увидели ваше объявление в {{source_name}}. Если вы ищете работу или подработку — заходите к нам на Podrabotki.by, там собраны актуальные вакансии.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Открыть вакансии</a>`,

	`Здравствуйте!

Нашли ваше объявление в {{source_name}}: {{source}}

Если вы сейчас ищете работу — заходите к нам. На Podrabotki.by можно быстро посмотреть смены и сразу связаться с работодателем, без долгого поиска по каналам.

<a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,

	`Добрый день!

Увидели ваше объявление в {{source_name}}. Пишем, потому что вы, похоже, в поиске работы.

Если это так — заходите к нам на Podrabotki.by. Мы собрали вакансии и подработки в одном месте: можно выбрать смену, посмотреть условия и написать работодателю напрямую.

Будет полезно, если не хочется каждый раз листать десятки Telegram-каналов.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Посмотреть вакансии</a>`,

	`Здравствуйте!

Увидели ваше объявление в {{source_name}} ({{source}}).

Если вы ищете работу или подработку — заходите к нам на Podrabotki.by. Там собраны свежие предложения по Минску и другим городам: смены, подработки, вакансии на постоянку.

Можно отфильтровать по графику, посмотреть детали и сразу откликнуться. Сервис бесплатный для соискателей.

Попробуйте — возможно, подходящая смена уже есть.

<a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Podrabotki.by</a>`,

	`Добрый день!

Увидели ваше объявление в {{source_name}} и решили написать.

Если вы сейчас в поиске работы — заходите к нам. Мы сделали Podrabotki.by как единое место, где можно найти подработку без бесконечного скролла по разным каналам и форумам.

Что там есть:
— актуальные вакансии и смены
— прямой контакт с работодателем
— удобный поиск по типу работы и графику

Объявление, которое мы увидели: {{source}}

Если тема поиска работы для вас актуальна — будем рады, если заглянете.

👉 <a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Перейти на Podrabotki.by</a>`,

	`Здравствуйте!

Пишем по поводу вашего объявления в {{source_name}}.

Мы увидели, что вы ищете работу, и хотели рассказать о сервисе, который может помочь быстрее найти подходящую смену.

Podrabotki.by — это платформа, где собраны вакансии из разных источников в одном месте. Не нужно вручную мониторить каналы, форумы и чаты: можно открыть сайт, посмотреть свежие предложения, сравнить условия и сразу связаться с работодателем.

Если поиск работы для вас сейчас в приоритете — заходите к нам, посмотрите, что есть на сегодня. Возможно, найдёте вариант лучше, чем ожидали.

Ссылка на ваше объявление: {{source}}

<a href="https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite">Открыть Podrabotki.by</a>`,
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
