package onliner

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

type TopicRef struct {
	ID    int
	Title string
}

type Topic struct {
	ID               int
	Title            string
	Body             string
	PosterUserID     string
	PosterUsername   string
	PosterProfileURL string
	Link             string
}

var (
	topicLinkRE = regexp.MustCompile(`viewtopic\.php\?t=(\d+)`)
	titleRE     = regexp.MustCompile(`(?is)<h2[^>]*class="wraptxt"[^>]*>\s*<a[^>]*href="[^"]*viewtopic\.php\?t=(\d+)"[^>]*>(.*?)</a>`)
	descRE      = regexp.MustCompile(`(?is)<p[^>]*class="ba-description"[^>]*>(.*?)</p>`)
	firstAuthorRE = regexp.MustCompile(`(?is)<div[^>]*class="b-mtauthor"[^>]*data-user_id="(\d+)"`)
	posterNickRE  = regexp.MustCompile(`(?is)<div[^>]*class="b-mtauthor"[^>]*data-user_id="(\d+)".*?<a[^>]*class="[^"]*_name[^"]*"[^>]*(?:title="([^"]*)")?[^>]*>([^<]*)</a>`)
	firstContentRE = regexp.MustCompile(`(?is)<li[^>]*class="[^"]*msgfirst[^"]*"[^>]*>.*?<div[^>]*class="content"[^>]*id="message_\d+"[^>]*>(.*?)</div>`)
	signatureRE = regexp.MustCompile(`(?is)<div[^>]*class="msgpost-signature"[^>]*id="sig\d+"[^>]*>(.*?)</div>`)
	pageTitleRE = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	tagRE       = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRE     = regexp.MustCompile(`\s+`)
)

func parseTopicRefs(pageHTML string) []TopicRef {
	seen := make(map[int]TopicRef)

	for _, m := range titleRE.FindAllStringSubmatch(pageHTML, -1) {
		id, err := strconv.Atoi(m[1])
		if err != nil || id <= 0 {
			continue
		}
		seen[id] = TopicRef{ID: id, Title: stripHTML(m[2])}
	}

	if len(seen) == 0 {
		for _, m := range topicLinkRE.FindAllStringSubmatch(pageHTML, -1) {
			id, err := strconv.Atoi(m[1])
			if err != nil || id <= 0 {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = TopicRef{ID: id}
			}
		}
	}

	return refsFromMap(seen)
}

func parseTopicPage(pageHTML string, topicID int) (Topic, error) {
	title := extractPageTitle(pageHTML)
	body := ""
	if m := firstContentRE.FindStringSubmatch(pageHTML); len(m) > 1 {
		body = stripHTML(m[1])
	}
	if body == "" {
		return Topic{}, fmt.Errorf("topic %d: empty body", topicID)
	}
	if m := signatureRE.FindStringSubmatch(pageHTML); len(m) > 1 {
		sig := strings.TrimSpace(stripHTML(m[1]))
		if sig != "" {
			body = strings.TrimSpace(body + "\n" + sig)
		}
	}

	posterID := ""
	posterName := ""
	if m := posterNickRE.FindStringSubmatch(pageHTML); len(m) > 1 {
		posterID = strings.TrimSpace(m[1])
		if len(m) > 3 {
			posterName = strings.TrimSpace(stripHTML(m[3]))
		}
		if posterName == "" && len(m) > 2 {
			posterName = strings.TrimSpace(m[2])
		}
	} else if m := firstAuthorRE.FindStringSubmatch(pageHTML); len(m) > 1 {
		posterID = m[1]
	}
	if posterName == "" {
		posterName = posterID
	}

	profileURL := ""
	if posterID != "" {
		profileURL = fmt.Sprintf("https://profile.onliner.by/user/%s", posterID)
	}

	return Topic{
		ID:               topicID,
		Title:            title,
		Body:             body,
		PosterUserID:     posterID,
		PosterUsername:   posterName,
		PosterProfileURL: profileURL,
		Link:             fmt.Sprintf("%s/viewtopic.php?t=%d", baseURL, topicID),
	}, nil
}

func extractPageTitle(pageHTML string) string {
	m := pageTitleRE.FindStringSubmatch(pageHTML)
	if len(m) < 2 {
		return ""
	}
	title := stripHTML(m[1])
	title = strings.TrimSuffix(title, " - Барахолка onliner.by")
	return strings.TrimSpace(title)
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func topicSearchText(topic Topic) string {
	return strings.TrimSpace(topic.Title + "\n" + topic.Body)
}
