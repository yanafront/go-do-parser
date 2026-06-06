package viber

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/anadubesko/go-do-parser/internal/post"
)

type Publisher struct {
	client     *Client
	fromUserID string
	channelID  string
}

func NewPublisher(authToken, fromUserID, webhookURL string, mux *http.ServeMux) (*Publisher, error) {
	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		return nil, fmt.Errorf("VIBER_AUTH_TOKEN is empty")
	}

	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, fmt.Errorf("VIBER_WEBHOOK_URL is required")
	}

	client := NewClient(authToken)

	if mux != nil {
		mux.HandleFunc("/viber/webhook", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":0}`))
		})
	}

	if err := client.SetWebhook(webhookURL); err != nil {
		return nil, fmt.Errorf("viber set_webhook: %w", err)
	}

	info, err := client.GetAccountInfo()
	if err != nil {
		return nil, fmt.Errorf("viber get_account_info: %w", err)
	}
	if info.Status != 0 {
		return nil, fmt.Errorf("viber account: %s (%d)", info.StatusMessage, info.Status)
	}

	fromUserID = strings.TrimSpace(fromUserID)
	if fromUserID == "" {
		fromUserID = SuperAdminID(info)
	}
	if fromUserID == "" {
		return nil, fmt.Errorf("VIBER_FROM_USER_ID is required (superadmin not found in channel)")
	}

	return &Publisher{
		client:     client,
		fromUserID: fromUserID,
		channelID:  info.ID,
	}, nil
}

func (p *Publisher) Name() string {
	return "viber"
}

func (p *Publisher) Destination() string {
	return p.channelID
}

func (p *Publisher) Publish(item post.Post, _ string) (int64, error) {
	text := post.PlainText(item)
	if text == "" {
		return 0, fmt.Errorf("empty text")
	}
	if len(text) > 7000 {
		text = text[:7000]
	}
	return p.client.PostText(p.fromUserID, text)
}
