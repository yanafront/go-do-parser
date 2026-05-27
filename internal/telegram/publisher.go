package telegram

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Publisher struct {
	bot      *tgbotapi.BotAPI
	destChat tgbotapi.BaseChat
}

func NewPublisher(token, destination string) (*Publisher, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	dest := strings.TrimSpace(destination)
	p := &Publisher{bot: bot}

	if strings.HasPrefix(dest, "@") {
		p.destChat = tgbotapi.BaseChat{
			ChannelUsername: strings.TrimPrefix(dest, "@"),
		}
		return p, nil
	}

	var id int64
	if _, err := fmt.Sscanf(dest, "%d", &id); err == nil && id != 0 {
		p.destChat = tgbotapi.BaseChat{ChatID: id}
		return p, nil
	}

	p.destChat = tgbotapi.BaseChat{ChannelUsername: dest}
	return p, nil
}

func (p *Publisher) Destination() string {
	if p.destChat.ChannelUsername != "" {
		return "@" + p.destChat.ChannelUsername
	}
	return fmt.Sprintf("%d", p.destChat.ChatID)
}

func (p *Publisher) ValidateAccess() error {
	_, err := p.bot.GetChat(tgbotapi.ChatInfoConfig{BaseChat: p.destChat})
	if err != nil {
		return fmt.Errorf("bot cannot access destination %s: %w (add bot as channel admin with post permission)", p.Destination(), err)
	}
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

	msg := tgbotapi.MessageConfig{
		BaseChat:              p.destChat,
		Text:                  text,
		ParseMode:             tgbotapi.ModeHTML,
		DisableWebPagePreview: false,
	}

	sent, err := p.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (p *Publisher) sendMedia(post Post, mediaPath string) (int, error) {
	caption := formatCaption(post)
	file := tgbotapi.FilePath(mediaPath)

	switch post.MediaKind {
	case "photo":
		photo := tgbotapi.PhotoConfig{
			BaseFile: tgbotapi.BaseFile{
				BaseChat: p.destChat,
				File:     file,
			},
			Caption:   caption,
			ParseMode: tgbotapi.ModeHTML,
		}
		sent, err := p.bot.Send(photo)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "video":
		video := tgbotapi.VideoConfig{
			BaseFile: tgbotapi.BaseFile{
				BaseChat: p.destChat,
				File:     file,
			},
			Caption:   caption,
			ParseMode: tgbotapi.ModeHTML,
		}
		sent, err := p.bot.Send(video)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	case "animation":
		anim := tgbotapi.AnimationConfig{
			BaseFile: tgbotapi.BaseFile{
				BaseChat: p.destChat,
				File:     file,
			},
			Caption:   caption,
			ParseMode: tgbotapi.ModeHTML,
		}
		sent, err := p.bot.Send(anim)
		if err != nil {
			return 0, err
		}
		return sent.MessageID, nil
	default:
		doc := tgbotapi.DocumentConfig{
			BaseFile: tgbotapi.BaseFile{
				BaseChat: p.destChat,
				File:     file,
			},
			Caption:   caption,
			ParseMode: tgbotapi.ModeHTML,
		}
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
