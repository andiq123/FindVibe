package domain

import "testing"

func TestNewProviderResultSetsSongProvider(t *testing.T) {
	song := NewSong("Hello", "Adele", "", "https://example.com/a.mp3")
	got := NewProviderResult(*song, "Mp3mn", 1, nil)
	if got.Song.Provider != "Mp3mn" || got.Provider != "Mp3mn" {
		t.Fatalf("provider not set: %#v", got)
	}
}
