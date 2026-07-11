package onliner

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://baraholka.onliner.by"

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Fetch(path string) (string, error) {
	target := path
	if !strings.HasPrefix(path, "http") {
		target = baseURL + path
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; go-do-parser/1.0)")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")

	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onliner fetch %s: status %d", target, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) FetchForum(forumID, start, pages int) ([]TopicRef, error) {
	if pages <= 0 {
		pages = 1
	}
	seen := make(map[int]TopicRef)
	for page := 0; page < pages; page++ {
		offset := start + page*50
		html, err := c.Fetch(fmt.Sprintf("/viewforum.php?f=%d&start=%d", forumID, offset))
		if err != nil {
			return nil, err
		}
		for _, ref := range parseTopicRefs(html) {
			seen[ref.ID] = ref
		}
	}
	return refsFromMap(seen), nil
}

func (c *Client) FetchSearch(query string, pages int) ([]TopicRef, error) {
	if pages <= 0 {
		pages = 1
	}
	seen := make(map[int]TopicRef)
	for page := 0; page < pages; page++ {
		offset := page * 50
		path := "/search.php?q=" + url.QueryEscape(query)
		if offset > 0 {
			path += fmt.Sprintf("&start=%d", offset)
		}
		html, err := c.Fetch(path)
		if err != nil {
			return nil, err
		}
		for _, ref := range parseTopicRefs(html) {
			seen[ref.ID] = ref
		}
	}
	return refsFromMap(seen), nil
}

func (c *Client) FetchTopic(topicID int) (Topic, error) {
	html, err := c.Fetch(fmt.Sprintf("/viewtopic.php?t=%d", topicID))
	if err != nil {
		return Topic{}, err
	}
	return parseTopicPage(html, topicID)
}

func refsFromMap(m map[int]TopicRef) []TopicRef {
	out := make([]TopicRef, 0, len(m))
	for _, ref := range m {
		out = append(out, ref)
	}
	return out
}
