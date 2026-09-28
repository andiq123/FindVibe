package services

import (
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"testing"
)

func TestCoreTitleStripsRemixVariants(t *testing.T) {
	a := CoreTitle("Collide (Extended Mix)")
	b := CoreTitle("Collide feat Rosi Golan [Extended Mix]")
	c := CoreTitle("Collide (Original Mix)")
	if a != "collide" || b != "collide" || c != "collide" {
		t.Fatalf("got %q %q %q", a, b, c)
	}
}

func TestSongKeyStable(t *testing.T) {
	if SongKey("Vicetone", "Collide (Extended Mix)") != SongKey("vicetone", "Collide") {
		t.Fatal("song keys should collapse remix variants")
	}
}

func TestMatchRejectsWrongRecording(t *testing.T) {
	for _, tc := range []struct{ artist, title string }{
		{"Adele", "Hello From The Other Side"},
		{"Adele", "Hello (Hardstyle Remix)"},
		{"Adele (Nightcore by DJ)", "Hello"},
		{"NotAdele", "Hello"},
	} {
		if IsPlayableMatch("Adele", "Hello", domain.Song{Artist: tc.artist, Title: tc.title, Link: "https://audio.test/song.mp3"}) {
			t.Errorf("accepted wrong recording: %+v", tc)
		}
	}
	for _, title := range []string{"Hello", "Hello (Remastered)", "Hello [Explicit]"} {
		if !IsPlayableMatch("Adele", "Hello", domain.Song{Artist: "Adele", Title: title, Link: "https://audio.test/song.mp3"}) {
			t.Errorf("rejected %s", title)
		}
	}
	if !IsPlayableMatch("Adele", "Hello (Live)", domain.Song{Artist: "Adele", Title: "Hello (Live)", Link: "https://audio.test/song.mp3"}) {
		t.Fatal("explicit version rejected")
	}
}
