package telegram

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Publisher struct {
	bot         *tgbotapi.BotAPI
	destination int64
	destChat    string
}

func NewPublisher(token, destination string) (*Publisher, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	dest := strings.TrimSpace(destination)
	p := &Publisher{
		bot:      bot,
		destChat: dest,
	}

	if strings.HasPrefix(dest, "@") {
		return p, nil
	}

	var id int64
	if _, err := fmt.Sscanf(dest, "%d", &id); err == nil && id != 0 {
		p.destination = id
	}

	return p, nil
}

func (p *Publisher) Publish(post Post, mediaPath string) (int, error) {
	chatID := p.chatID()

	if post.HasMedia && mediaPath != "" {
		return p.sendMedia(chatID, post, mediaPath)
	}

	text := formatText(post)
	if text == "" {
		return 0, fmt.Errorf("empty post")
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = false

	sent, err := p.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (p *Publisher) sendMedia(chatID interface{}, post Post, mediaPath string) (int, error) {
	caption := formatCaption(post)

	switch post.MediaKind {
	case "photo":
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(mediaPath))
		photo.Caption = caption
		photo.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(photo)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "video":
		video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(mediaPath))
		video.Caption = caption
		video.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(video)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "animation":
		anim := tgbotapi.NewAnimation(chatID, tgbotapi.FilePath(mediaPath))
		anim.Caption = caption
		anim.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(anim)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	default:
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(mediaPath))
		doc.Caption = caption
		doc.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(doc)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	}
}

func (p *Publisher) chatID() interface{} {
	if p.destination != 0 {
		return p.destination
	}
	return p.destChat
}

func formatText(post Post) string {
	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	if text == "" {
		return sourceTag(post)
	}
	return sourceTag(post) + "\n\n" + escapeHTML(text)
}

func formatCaption(post Post) string {
	text := strings.TrimSpace(post.Caption)
	if text == "" {
		text = strings.TrimSpace(post.Text)
	}
	if text == "" {
		return sourceTag(post)
	}
	return sourceTag(post) + "\n\n" + escapeHTML(text)
}

func sourceTag(post Post) string {
	channel := normalizeUsername(post.SourceChannel)
	return fmt.Sprintf("<i>@%s</i>", escapeHTML(channel))
}

func escapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

func TempMediaPath(dataDir string, channel string, messageID int) string {
	name := fmt.Sprintf("%s_%d", normalizeUsername(channel), messageID)
	return filepath.Join(dataDir, "tmp", name)
}

func CleanupMedia(path string) {
	_ = os.Remove(path)
}
