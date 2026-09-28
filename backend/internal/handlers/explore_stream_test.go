package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

func TestExploreStreamEventNDJSON(t *testing.T) {
	cached := false
	sec := ExploreSection{
		ID: "romania", Title: "Romania", Subtitle: "Charts at home",
		Songs: []domain.Song{{Id: "1", Title: "A", Artist: "B", Link: "https://x/a.mp3"}},
	}
	lines := []exploreStreamEvent{
		{Type: "meta", Country: "Romania", Cached: &cached},
		{Type: "section", Section: &sec},
		{Type: "done", Cached: &cached},
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	for _, ev := range lines {
		if err := enc.Encode(ev); err != nil {
			t.Fatal(err)
		}
	}
	raw := buf.String()
	parts := strings.Split(strings.TrimSpace(raw), "\n")
	if len(parts) != 3 {
		t.Fatalf("lines=%d raw=%q", len(parts), raw)
	}
	var again exploreStreamEvent
	if err := json.Unmarshal([]byte(parts[1]), &again); err != nil {
		t.Fatal(err)
	}
	if again.Type != "section" || again.Section == nil || again.Section.ID != "romania" {
		t.Fatalf("got %#v", again)
	}
}
