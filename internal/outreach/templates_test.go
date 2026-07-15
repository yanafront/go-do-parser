package outreach

import (
	"strings"
	"testing"
)

func TestFillSourcePlaceholders(t *testing.T) {
	got := FillSourcePlaceholders("Источник {{source_name}}: {{source}}", "https://t.me/x/1", "@x")
	if got != "Источник @x: https://t.me/x/1" {
		t.Fatalf("got %q", got)
	}
}

func TestPickSeekerMessage(t *testing.T) {
	msg := PickSeekerMessage("https://t.me/ch/10", "@ch")
	if strings.TrimSpace(msg) == "" {
		t.Fatal("empty message")
	}
	if !strings.Contains(msg, "https://t.me/ch/10") && !strings.Contains(msg, "@ch") {
		t.Fatalf("source not injected: %q", msg)
	}
}

func TestFormatSourceLink(t *testing.T) {
	if got := FormatSourceLink("PodrabotkaRabota", "", 123); got != "https://t.me/PodrabotkaRabota/123" {
		t.Fatalf("got %q", got)
	}
	if got := FormatSourceLink("onliner:34", "", 26198835); got != "https://baraholka.onliner.by/viewtopic.php?t=26198835" {
		t.Fatalf("got %q", got)
	}
}
