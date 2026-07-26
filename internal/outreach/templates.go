package outreach

import (
	"fmt"
	"math/rand"
	"strings"
)

const sourcePlaceholder = "{{source}}"
const sourceNamePlaceholder = "{{source_name}}"

const platformURL = "https://podrabotki.by/?utm_source=telegram_dm&utm_medium=message&utm_campaign=seeker_invite"

var seekerTemplates = []string{
	`Здравствуйте!

Увидели ваше объявление в {{source_name}} и подумали, что вам может быть полезно.

На <a href="` + platformURL + `">Podrabotki.by</a> собраны подработки и смены в одном месте — можно быстро посмотреть варианты и сразу написать работодателю.

Если сейчас не в поиске — просто проигнорируйте сообщение, без обид 🙂`,

	`Добрый день!

Пишу по вашему объявлению из {{source_name}}.

Если вы ищете работу или смену на ближайшие дни — загляните на <a href="` + platformURL + `">Podrabotki.by</a>. Там удобно смотреть свежие предложения без долгого листать каналы.

Вдруг пригодится!`,

	`Здравствуйте!

Наткнулись на ваше объявление в {{source_name}}:
{{source}}

Сделали сервис <a href="` + platformURL + `">Podrabotki.by</a> — чтобы быстрее находить подработку и вакансии. Для соискателей бесплатно.

Если актуально — будем рады, если заглянете. Если нет — извините за беспокойство.`,

	`Добрый день!

Увидели, что вы, похоже, в поиске работы (объявление в {{source_name}}).

Может быть полезно: на <a href="` + platformURL + `">Podrabotki.by</a> можно выбрать смену, посмотреть условия и связаться с работодателем напрямую.

Ни к чему не обязывает — просто вариант, если сейчас ищете 🙂`,

	`Здравствуйте!

Коротко по делу: увидели ваше объявление в {{source_name}} и хотели поделиться ссылкой на <a href="` + platformURL + `">Podrabotki.by</a>.

Там собраны актуальные подработки — иногда получается быстрее, чем искать по разным чатам.

Если тема неактуальна — спокойно пропустите это сообщение.`,

	`Добрый день!

Ваше объявление в {{source_name}} попалось нам на глаза.

Если сейчас ищете работу, попробуйте <a href="` + platformURL + `">Podrabotki.by</a>: свежие смены, фильтры по графику, прямой контакт с работодателем.

Надеемся, будет полезно. Хорошего дня!`,

	`Здравствуйте!

Пишем не с рекламой «в лоб», а потому что увидели ваше объявление в {{source_name}}.

На <a href="` + platformURL + `">Podrabotki.by</a> можно быстро посмотреть, какие смены есть сейчас, и сразу откликнуться. Сервис для соискателей бесплатный.

Если уже нашли работу — замечательно, тогда просто не обращайте внимания 🙂`,

	`Добрый день!

По вашему объявлению из {{source_name}} ({{source}}).

Если поиск работы ещё актуален — вот место, где удобно смотреть варианты: <a href="` + platformURL + `">Podrabotki.by</a>.

Без регистрации ради регистрации и без спама с нашей стороны дальше. Просто делимся инструментом, который может сэкономить время.`,

	`Здравствуйте!

Увидели ваше объявление в {{source_name}} и решили мягко подсказать.

Иногда проще открыть один сайт, чем скроллить десятки каналов. Поэтому сделали <a href="` + platformURL + `">Podrabotki.by</a> — подработки и вакансии в одном месте.

Если откликнется — супер. Если нет — всё ок, не хотели отвлечь зря.`,

	`Добрый день!

Ищем людей, которым сейчас нужна подработка или смена — и ваше объявление в {{source_name}} как раз об этом.

Загляните на <a href="` + platformURL + `">Podrabotki.by</a>: там можно посмотреть свежие предложения и сразу написать работодателю.

Если уже всё нашлось — примите извинения за сообщение.`,

	`Здравствуйте!

Небольшое сообщение по вашему объявлению в {{source_name}}.

Мы развиваем <a href="` + platformURL + `">Podrabotki.by</a> — платформу, где собраны вакансии и подработки. Хотели просто дать ссылку на случай, если сейчас в поиске.

Спасибо, что уделили минуту. Если неактуально — можно не отвечать.`,

	`Добрый день!

Увидели объявление в {{source_name}} и подумали: вдруг пригодится.

<a href="` + platformURL + `">Podrabotki.by</a> — это подбор смен и подработок без лишней суеты. Можно сравнить варианты и выбрать удобный график.

Желаем удачно найти то, что нужно — с нами или без 🙂`,
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
	templates := make([]string, 0, len(seekerTemplates))
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
