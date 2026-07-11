package onliner

import "testing"

func TestExtractContacts(t *testing.T) {
	text := "ищу работу звоните +375291442078 или mail@test.by @myuser123"
	c := ExtractContacts(text)
	if c.Phone == "" {
		t.Fatalf("expected phone")
	}
	if c.Email != "mail@test.by" {
		t.Fatalf("email=%q", c.Email)
	}
}
