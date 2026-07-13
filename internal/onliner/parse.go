package onliner

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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
	PostedAt         *time.Time
}

var (
	topicLinkRE = regexp.MustCompile(`viewtopic\.php\?t=(\d+)`)
	titleRE     = regexp.MustCompile(`(?is)<h2[^>]*class="wraptxt"[^>]*>\s*<a[^>]*href="[^"]*viewtopic\.php\?t=(\d+)"[^>]*>(.*?)</a>`)
	descRE      = regexp.MustCompile(`(?is)<p[^>]*class="ba-description"[^>]*>(.*?)</p>`)
	firstAuthorRE = regexp.MustCompile(`(?is)<div[^>]*class="b-mtauthor"[^>]*data-user_id="(\d+)"`)
	posterNickRE  = regexp.MustCompile(`(?is)<div[^>]*class="b-mtauthor"[^>]*data-user_id="(\d+)".*?<a[^>]*class="[^"]*_name[^"]*"[^>]*(?:title="([^"]*)")?[^>]*>([^<]*)</a>`)
	firstContentRE = regexp.MustCompile(`(?is)<li[^>]*class="[^"]*msgfirst[^"]*"[^>]*>.*?<div[^>]*class="content"[^>]*id="message_\d+"[^>]*>(.*?)</div>`)
	firstDateRE    = regexp.MustCompile(`(?is)<li[^>]*class="[^"]*msgfirst[^"]*"[^>]*>.*?<small[^>]*class="msgpost-date"[^>]*>.*?<span[^>]*title="[^"]*"[^>]*>([^<]+)</span>`)
	russianDateRE  = regexp.MustCompile(`(\d{1,2})\s+(\S+)\s+(\d{4})\s+(\d{1,2}):(\d{2})`)
	imgSrcRE      = regexp.MustCompile(`(?is)<img[^>]*src="([^"]+)"[^>]*>`)
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

	return sortRefsByIDDesc(seen)
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

	var postedAt *time.Time
	if m := firstDateRE.FindStringSubmatch(pageHTML); len(m) > 1 {
		if t, ok := parseRussianDateTime(m[1]); ok {
			postedAt = &t
		}
	}

	return Topic{
		ID:               topicID,
		Title:            title,
		Body:             body,
		PosterUserID:     posterID,
		PosterUsername:   posterName,
		PosterProfileURL: profileURL,
		Link:             fmt.Sprintf("%s/viewtopic.php?t=%d", baseURL, topicID),
		PostedAt:         postedAt,
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
	s = imgSrcRE.ReplaceAllString(s, "$1")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func topicSearchText(topic Topic) string {
	return strings.TrimSpace(topic.Title + "\n" + topic.Body)
}

var ruMonths = map[string]time.Month{
	"января":   time.January,
	"февраля":  time.February,
	"марта":    time.March,
	"апреля":   time.April,
	"мая":      time.May,
	"июня":     time.June,
	"июля":     time.July,
	"августа":  time.August,
	"сентября": time.September,
	"октября":  time.October,
	"ноября":   time.November,
	"декабря":  time.December,
}

func parseRussianDateTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	m := russianDateRE.FindStringSubmatch(raw)
	if len(m) != 6 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}, false
	}
	month, ok := ruMonths[strings.ToLower(m[2])]
	if !ok {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return time.Time{}, false
	}
	hour, err := strconv.Atoi(m[4])
	if err != nil {
		return time.Time{}, false
	}
	minute, err := strconv.Atoi(m[5])
	if err != nil {
		return time.Time{}, false
	}
	loc, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		loc = time.UTC
	}
	return time.Date(year, month, day, hour, minute, 0, 0, loc), true
}

func sortRefsByIDDesc(seen map[int]TopicRef) []TopicRef {
	out := make([]TopicRef, 0, len(seen))
	for _, ref := range seen {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID > out[j].ID
	})
	return out
}
