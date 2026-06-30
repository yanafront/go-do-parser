package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anadubesko/go-do-parser/internal/telegram"
)

type Client struct {
	url    string
	secret string
	client *http.Client
}

func New(url, secret string) *Client {
	return &Client{
		url:    strings.TrimRight(strings.TrimSpace(url), "/"),
		secret: strings.TrimSpace(secret),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.url != ""
}

type payload struct {
	SourceChannel string `json:"source_channel"`
	MessageID     int    `json:"message_id"`
	Text          string `json:"text"`
	MediaKind     string `json:"media_kind"`
}

func (c *Client) Notify(ctx context.Context, post telegram.Post) error {
	if !c.Enabled() {
		return nil
	}
	text := post.Text
	if text == "" {
		text = post.Caption
	}
	body, err := json.Marshal(payload{
		SourceChannel: post.SourceChannel,
		MessageID:     post.MessageID,
		Text:          text,
		MediaKind:     post.MediaKind,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/ingest", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Ingest-Secret", c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("matcher ingest status %d", resp.StatusCode)
	}
	return nil
}
