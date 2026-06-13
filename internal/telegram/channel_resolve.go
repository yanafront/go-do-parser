package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type channelResolver struct {
	api   *tg.Client
	cache map[string]*tg.Channel
	mu    sync.RWMutex
}

func newChannelResolver(api *tg.Client) *channelResolver {
	return &channelResolver{
		api:   api,
		cache: make(map[string]*tg.Channel),
	}
}

func (r *channelResolver) get(ctx context.Context, channelUsername string) (*tg.Channel, error) {
	username := normalizeUsername(channelUsername)
	if username == "" {
		return nil, fmt.Errorf("empty channel username")
	}

	r.mu.RLock()
	if ch, ok := r.cache[username]; ok {
		r.mu.RUnlock()
		return ch, nil
	}
	r.mu.RUnlock()

	ch, err := resolveChannel(ctx, r.api, username)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[username] = ch
	r.mu.Unlock()
	return ch, nil
}

func resolveChannel(ctx context.Context, api *tg.Client, username string) (*tg.Channel, error) {
	for _, name := range uniqueNames(username) {
		ch, err := resolveByUsername(ctx, api, name)
		if err == nil {
			return ch, nil
		}
	}

	ch, err := findChannelInDialogs(ctx, api, username)
	if err == nil {
		return ch, nil
	}

	return nil, fmt.Errorf("resolve @%s: channel not found in Telegram (check username in TG_SOURCES)", username)
}

func uniqueNames(username string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range []string{username, strings.ToLower(username)} {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func resolveByUsername(ctx context.Context, api *tg.Client, username string) (*tg.Channel, error) {
	resolved, err := api.ContactsResolveUsername(ctx, username)
	if err != nil {
		if rpcErr, ok := tgerr.As(err); ok {
			return nil, fmt.Errorf("contacts.resolveUsername(%q): %s", username, rpcErr.Type)
		}
		return nil, fmt.Errorf("contacts.resolveUsername(%q): %w", username, err)
	}
	return pickChannel(resolved.Chats, username)
}

func pickChannel(chats []tg.ChatClass, username string) (*tg.Channel, error) {
	for _, chat := range chats {
		ch, ok := chat.(*tg.Channel)
		if !ok {
			continue
		}
		if ch.Username == "" || strings.EqualFold(ch.Username, username) {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("channel @%s not in resolve result", username)
}

func findChannelInDialogs(ctx context.Context, api *tg.Client, username string) (*tg.Channel, error) {
	offsetDate := 0
	offsetID := 0
	offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})

	for page := 0; page < 15; page++ {
		dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      100,
		})
		if err != nil {
			return nil, err
		}

		var chats []tg.ChatClass
		var messages []*tg.Message
		switch v := dialogs.(type) {
		case *tg.MessagesDialogs:
			chats = v.Chats
			messages = filterMessages(v.Messages)
		case *tg.MessagesDialogsSlice:
			chats = v.Chats
			messages = filterMessages(v.Messages)
		default:
			return nil, fmt.Errorf("unexpected dialogs type %T", dialogs)
		}

		for _, chat := range chats {
			ch, ok := chat.(*tg.Channel)
			if !ok || ch.Username == "" {
				continue
			}
			if strings.EqualFold(ch.Username, username) {
				return ch, nil
			}
		}

		if len(messages) == 0 {
			break
		}
		last := messages[len(messages)-1]
		offsetDate = last.Date
		offsetID = last.ID
		offsetPeer, err = dialogPeer(last.PeerID, chats)
		if err != nil {
			break
		}
	}

	return nil, fmt.Errorf("not found in account dialogs")
}

func dialogPeer(peer tg.PeerClass, chats []tg.ChatClass) (tg.InputPeerClass, error) {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		for _, chat := range chats {
			ch, ok := chat.(*tg.Channel)
			if ok && ch.ID == p.ChannelID {
				return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}, nil
			}
		}
	case *tg.PeerUser:
		return &tg.InputPeerUser{UserID: p.UserID}, nil
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, nil
	}
	return &tg.InputPeerEmpty{}, fmt.Errorf("unknown peer")
}
