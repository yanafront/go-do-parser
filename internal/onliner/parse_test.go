package onliner

import (
	"testing"

	"github.com/anadubesko/go-do-parser/internal/filter"
)

func TestParseTopicRefs(t *testing.T) {
	html := `
<h2 class="wraptxt"><a href="./viewtopic.php?t=26205615">Ищу подработку</a></h2>
<p class="ba-description">48 лет ищу работу или подработку</p>
<h2 class="wraptxt"><a href="/viewtopic.php?t=26181106">Грузчик</a></h2>
`
	refs := parseTopicRefs(html)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	byID := make(map[int]TopicRef)
	for _, ref := range refs {
		byID[ref.ID] = ref
	}
	if byID[26205615].Title != "Ищу подработку" {
		t.Fatalf("unexpected ref 26205615: %+v", byID[26205615])
	}
}

func TestParseTopicPage(t *testing.T) {
	html := `
<li id="p116435003" class="msgpost msgfirst">
<div class="b-mtauthor" data-user_id="2591731">
<div class="content" id="message_111753550"><p>ищу работу подработку +375291442078</p></div>
<div class="msgpost-signature" id="sig111753550">ЮРИЙ</div>
`
	topic, err := parseTopicPage(html, 26205615)
	if err != nil {
		t.Fatal(err)
	}
	if topic.PosterUserID != "2591731" {
		t.Fatalf("poster=%q", topic.PosterUserID)
	}
	if !filter.IsJobSeekerText(topic.Title + "\n" + topic.Body) {
		t.Fatalf("expected job seeker text: %q", topic.Body)
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML("<p>ищу<br/>работу</p>")
	if got != "ищу работу" {
		t.Fatalf("got %q", got)
	}
}
