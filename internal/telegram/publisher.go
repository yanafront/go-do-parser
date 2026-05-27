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
	destChat    tgbotapi.BaseChat
	destination string
}

func NewPublisher(token, destination string) (*Publisher, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	dest := normalizeDestination(destination)
	p := &Publisher{
		bot:         bot,
		destination: dest,
	}

	if strings.HasPrefix(dest, "-") || (dest != "" && dest[0] >= '0' && dest[0] <= '9') {
		var id int64
		if _, err := fmt.Sscanf(dest, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid TG_DESTINATION id %q", destination)
		}
		p.destChat = tgbotapi.BaseChat{ChatID: id}
		return p, nil
	}

	username := strings.TrimPrefix(dest, "@")
	p.destChat = tgbotapi.BaseChat{ChannelUsername: username}
	return p, nil
}

func normalizeDestination(dest string) string {
	dest = strings.TrimSpace(dest)
	dest = strings.TrimPrefix(dest, "https://t.me/")
	dest = strings.TrimPrefix(dest, "http://t.me/")
	dest = strings.TrimPrefix(dest, "t.me/")
	return dest
}

func (p *Publisher) Destination() string {
	if p.destChat.ChatID != 0 {
		return fmt.Sprintf("%d", p.destChat.ChatID)
	}
	if p.destination != "" {
		if strings.HasPrefix(p.destination, "@") {
			return p.destination
		}
		return "@" + p.destination
	}
	return ""
}

func (p *Publisher) ChatID() int64 {
	return p.destChat.ChatID
}

func (p *Publisher) ValidateAccess() error {
	cfg := tgbotapi.ChatInfoConfig{}
	if p.destChat.ChatID != 0 {
		cfg.ChatID = p.destChat.ChatID
	} else {
		username := p.destChat.ChannelUsername
		if !strings.HasPrefix(username, "@") {
			username = "@" + username
		}
		cfg.SuperGroupUsername = username
	}

	chat, err := p.bot.GetChat(cfg)
	if err != nil {
		return fmt.Errorf("bot cannot access %s: %w (add bot as channel admin with post permission, or use numeric channel id in TG_DESTINATION)", p.Destination(), err)
	}
	p.destChat = tgbotapi.BaseChat{ChatID: chat.ID}
	return nil
}

func (p *Publisher) Publish(post Post, mediaPath string) (int, error) {
	if post.HasMedia && mediaPath != "" {
		return p.sendMedia(post, mediaPath)
	}

	text := formatText(post)
	if text == "" {
		return 0, fmt.Errorf("empty post")
	}

	msg := tgbotapi.NewMessage(p.destChat.ChatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = false

	sent, err := p.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (p *Publisher) sendMedia(post Post, mediaPath string) (int, error) {
	caption := formatCaption(post)
	file := tgbotapi.FilePath(mediaPath)
	chatID := p.destChat.ChatID

	switch post.MediaKind {
	case "photo":
		photo := tgbotapi.NewPhoto(chatID, file)
		photo.Caption = caption
		photo.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(photo)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "video":
		video := tgbotapi.NewVideo(chatID, file)
		video.Caption = caption
		video.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(video)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "animation":
		anim := tgbotapi.NewAnimation(chatID, file)
		anim.Caption = caption
		anim.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(anim)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	default:
		doc := tgbotapi.NewDocument(chatID, file)
		doc.Caption = caption
		doc.ParseMode = tgbotapi.ModeHTML
		sent, err := p.bot.Send(doc)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	}
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
