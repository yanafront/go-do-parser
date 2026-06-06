package viber

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://chatapi.viber.com/pa"

type Client struct {
	authToken string
	http      *http.Client
}

type apiResponse struct {
	Status        int    `json:"status"`
	StatusMessage string `json:"status_message"`
	MessageToken  int64  `json:"message_token"`
}

type AccountInfo struct {
	Status        int      `json:"status"`
	StatusMessage string   `json:"status_message"`
	ID            string   `json:"Id"`
	Name          string   `json:"chat_hostname"`
	Members       []Member `json:"members"`
}

type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func NewClient(authToken string) *Client {
	return &Client{
		authToken: authToken,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SetWebhook(url string) error {
	var resp apiResponse
	if err := c.call("/set_webhook", map[string]string{
		"auth_token": c.authToken,
		"url":        url,
	}, &resp); err != nil {
		return err
	}
	if resp.Status != 0 {
		return fmt.Errorf("set_webhook: %s (%d)", resp.StatusMessage, resp.Status)
	}
	return nil
}

func (c *Client) GetAccountInfo() (*AccountInfo, error) {
	var info AccountInfo
	if err := c.call("/get_account_info", map[string]string{
		"auth_token": c.authToken,
	}, &info); err != nil {
		return nil, err
	}
	if info.Status != 0 {
		return nil, fmt.Errorf("get_account_info: %s (%d)", info.StatusMessage, info.Status)
	}
	return &info, nil
}

func (c *Client) PostText(fromUserID, text string) (int64, error) {
	var resp apiResponse
	if err := c.call("/post", map[string]interface{}{
		"auth_token": c.authToken,
		"from":       fromUserID,
		"type":       "text",
		"text":       text,
	}, &resp); err != nil {
		return 0, err
	}
	if resp.Status != 0 {
		return 0, fmt.Errorf("post: %s (%d)", resp.StatusMessage, resp.Status)
	}
	return resp.MessageToken, nil
}

func (c *Client) call(path string, body interface{}, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiBase+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("viber http %d: %s", res.StatusCode, string(raw))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func SuperAdminID(info *AccountInfo) string {
	for _, m := range info.Members {
		if m.Role == "superadmin" {
			return m.ID
		}
	}
	if len(info.Members) > 0 {
		return info.Members[0].ID
	}
	return ""
}
