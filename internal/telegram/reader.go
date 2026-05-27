package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

type Reader struct {
	apiID       int
	apiHash     string
	phone       string
	dataDir     string
	log         *zap.Logger
	client      *telegram.Client
	api         *tg.Client
	downloader  *downloader.Downloader
	ready       bool
}

func NewReader(apiID int, apiHash, phoneNum, dataDir string, log *zap.Logger) *Reader {
	return &Reader{
		apiID:   apiID,
		apiHash: apiHash,
		phone:   phoneNum,
		dataDir: dataDir,
		log:     log,
	}
}

func NormalizeChannelKey(s string) string {
	return normalizeUsername(s)
}

func (r *Reader) Connect(ctx context.Context, ready chan<- struct{}) error {
	sessionPath, err := prepareSession(r.dataDir)
	if err != nil {
		return err
	}

	if os.Getenv("TG_SESSION") != "" {
		r.log.Info("telegram session restored from TG_SESSION env")
	} else if sessionExists(sessionPath) {
		r.log.Info("telegram session loaded from disk", zap.String("path", sessionPath))
	} else {
		onRailway := os.Getenv("RAILWAY_ENVIRONMENT") != ""
		hasVolume := os.Getenv("RAILWAY_VOLUME_MOUNT_PATH") != ""
		hasSessionEnv := os.Getenv("TG_SESSION") != ""

		fields := []zap.Field{
			zap.String("data_dir", r.dataDir),
			zap.Bool("data_dir_writable", dataDirWritable(r.dataDir)),
		}
		if onRailway && !hasVolume && !hasSessionEnv {
			fields = append(fields, zap.String("hint", "add Volume via canvas (right-click service → Add Volume, path /app/data) OR set TG_SESSION"))
		} else {
			fields = append(fields, zap.String("hint", "set TG_AUTH_CODE to the latest Telegram code, redeploy once"))
		}
		r.log.Warn("telegram session not found", fields...)
	}

	sessionStorage := &session.FileStorage{Path: sessionPath}

	r.client = telegram.NewClient(r.apiID, r.apiHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	return r.client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(
			newEnvAuthenticator(
				r.phone,
				strings.TrimSpace(os.Getenv("TG_AUTH_PASSWORD")),
				func(msg string) { r.log.Warn(msg) },
			),
			auth.SendCodeOptions{},
		)
		if err := r.client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		r.log.Info("telegram authorized")

		r.api = r.client.API()
		r.downloader = downloader.NewDownloader()
		r.ready = true

		close(ready)
		return r.runUntilCancel(ctx)
	})
}

func (r *Reader) runUntilCancel(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (r *Reader) FetchNewPosts(ctx context.Context, channelUsername string, afterID int, limit int) ([]Post, error) {
	return r.fetchPosts(ctx, channelUsername, afterID, 0, limit, true)
}

func (r *Reader) FetchHistoricalPage(ctx context.Context, channelUsername string, offsetID int, limit int) ([]Post, error) {
	return r.fetchPosts(ctx, channelUsername, 0, offsetID, limit, false)
}

func (r *Reader) fetchPosts(ctx context.Context, channelUsername string, minID, offsetID int, limit int, useMinID bool) ([]Post, error) {
	if !r.ready || r.api == nil {
		return nil, fmt.Errorf("reader not connected")
	}

	username := normalizeUsername(channelUsername)
	resolved, err := r.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("resolve @%s: %w", username, err)
	}

	var channel *tg.Channel
	for _, chat := range resolved.Chats {
		if ch, ok := chat.(*tg.Channel); ok {
			channel = ch
			break
		}
	}
	if channel == nil {
		return nil, fmt.Errorf("channel @%s not found", username)
	}

	req := &tg.MessagesGetHistoryRequest{
		Peer:  &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash},
		Limit: limit,
	}
	if useMinID {
		req.MinID = minID
	} else {
		req.OffsetID = offsetID
	}

	history, err := r.api.MessagesGetHistory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get history @%s: %w", username, err)
	}

	messages := extractMessages(history)
	posts := make([]Post, 0, len(messages))
	for _, msg := range messages {
		if useMinID && msg.ID <= minID {
			continue
		}
		post, ok := messageToPost(username, msg)
		if !ok {
			continue
		}
		post.SourceChannel = channelUsername
		posts = append(posts, post)
	}

	for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
		posts[i], posts[j] = posts[j], posts[i]
	}

	return posts, nil
}

func (r *Reader) DownloadMedia(ctx context.Context, channelUsername string, messageID int, destPath string) error {
	if !r.ready || r.api == nil {
		return fmt.Errorf("reader not connected")
	}

	username := normalizeUsername(channelUsername)
	resolved, err := r.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("resolve @%s: %w", username, err)
	}

	var channel *tg.Channel
	for _, chat := range resolved.Chats {
		if ch, ok := chat.(*tg.Channel); ok {
			channel = ch
			break
		}
	}
	if channel == nil {
		return fmt.Errorf("channel @%s not found", username)
	}

	msgs, err := r.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: &tg.InputChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash},
		ID: []tg.InputMessageClass{
			&tg.InputMessageID{ID: messageID},
		},
	})
	if err != nil {
		return returnDownloadErr(err)
	}

	for _, msg := range extractMessages(msgs) {
		if msg.ID != messageID {
			continue
		}
		loc, fileName, ok := mediaLocation(msg)
		if !ok {
			return fmt.Errorf("message %d has no downloadable media", messageID)
		}
		f, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = r.downloader.Download(r.api, loc).Stream(ctx, f)
		if err != nil {
			return err
		}
		_ = fileName
		return nil
	}

	return fmt.Errorf("message %d not found", messageID)
}

func extractMessages(box tg.MessagesMessagesClass) []*tg.Message {
	switch v := box.(type) {
	case *tg.MessagesMessages:
		return filterMessages(v.Messages)
	case *tg.MessagesMessagesSlice:
		return filterMessages(v.Messages)
	case *tg.MessagesChannelMessages:
		return filterMessages(v.Messages)
	default:
		return nil
	}
}

func filterMessages(items []tg.MessageClass) []*tg.Message {
	out := make([]*tg.Message, 0, len(items))
	for _, item := range items {
		if msg, ok := item.(*tg.Message); ok {
			out = append(out, msg)
		}
	}
	return out
}

func messageToPost(channelKey string, msg *tg.Message) (Post, bool) {
	if msg.Out {
		return Post{}, false
	}

	post := Post{
		SourceChannel: channelKey,
		MessageID:     msg.ID,
		Text:          msg.Message,
	}

	if msg.GroupedID != 0 {
		post.GroupedID = msg.GroupedID
	}

	switch media := msg.Media.(type) {
	case nil:
		if strings.TrimSpace(post.Text) == "" {
			return Post{}, false
		}
	case *tg.MessageMediaPhoto:
		post.HasMedia = true
		post.MediaKind = "photo"
		if msg.Message != "" {
			post.Caption = msg.Message
			post.Text = ""
		}
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return Post{}, false
		}
		post.HasMedia = true
		post.MediaKind = documentKind(doc)
		if msg.Message != "" {
			post.Caption = msg.Message
			post.Text = ""
		}
	default:
		if strings.TrimSpace(post.Text) == "" {
			return Post{}, false
		}
	}

	return post, true
}

func documentKind(doc *tg.Document) string {
	for _, attr := range doc.Attributes {
		switch attr.(type) {
		case *tg.DocumentAttributeVideo:
			return "video"
		case *tg.DocumentAttributeAnimated:
			return "animation"
		}
	}
	if doc.MimeType == "video/mp4" {
		return "video"
	}
	return "document"
}

func mediaLocation(msg *tg.Message) (tg.InputFileLocationClass, string, bool) {
	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok || len(photo.Sizes) == 0 {
			return nil, "", false
		}
		size := largestPhotoSize(photo.Sizes)
		if size == nil {
			return nil, "", false
		}
		switch s := size.(type) {
		case *tg.PhotoSizeProgressive:
			return &tg.InputPhotoFileLocation{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
				ThumbSize:     s.Type,
			}, "photo.jpg", true
		case *tg.PhotoSize:
			return &tg.InputPhotoFileLocation{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
				ThumbSize:     s.Type,
			}, "photo.jpg", true
		default:
			return nil, "", false
		}
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil, "", false
		}
		return &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}, "file", true
	default:
		return nil, "", false
	}
}

func largestPhotoSize(sizes []tg.PhotoSizeClass) tg.PhotoSizeClass {
	var best tg.PhotoSizeClass
	var bestArea int
	for _, s := range sizes {
		switch v := s.(type) {
		case *tg.PhotoSize:
			area := v.W * v.H
			if area > bestArea {
				bestArea = area
				best = v
			}
		case *tg.PhotoSizeProgressive:
			area := v.W * v.H
			if area > bestArea {
				bestArea = area
				best = v
			}
		}
	}
	return best
}

func normalizeUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://t.me/")
	s = strings.TrimPrefix(s, "t.me/")
	s = strings.TrimPrefix(s, "@")
	return s
}

func returnDownloadErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("download: %w", err)
}
