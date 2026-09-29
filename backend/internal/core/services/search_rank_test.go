package services

import (
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"testing"
)

func TestSearchRankingPreservesVersionsAndAlternates(t *testing.T) {
	songs := []domain.Song{
		{Artist: "Alex Velea", Title: "Other", Link: "https://mp3.pm/other", Provider: "Mp3pm"},
		{Artist: "Dani Mocanu, Alex Velea", Title: "Cariceps", Link: "https://mp3.pm/a", Provider: "Mp3pm"},
		{Artist: "Dani Mocanu & Alex Velea", Title: "Cariceps", Link: "https://new.kachevo.org/a", Provider: "MusicBoss"},
		{Artist: "Dani Mocanu, Alex Velea", Title: "Cariceps (Live)", Link: "https://mp3.pm/live", Provider: "Mp3pm"},
	}
	got := rankSearchResults("alex velea cariceps", songs, 20)
	if len(got) != 2 || got[0].Provider != "MusicBoss" || len(got[0].Alternatives) != 1 || got[1].Title != "Cariceps (Live)" {
		t.Fatalf("bad results: %+v", got)
	}
	got[0].Alternatives[0].Link = "original"
	cloned := cloneSearchResponse(domain.NewSearchResponse(got, nil))
	cloned.Songs[0].Alternatives[0].Link = "changed"
	if got[0].Alternatives[0].Link != "original" {
		t.Fatal("shared alternatives")
	}
}

func TestSearchCyrillicAndDifferentArtists(t *testing.T) {
	songs := []domain.Song{{Artist: "Кино", Title: "Группа крови", Link: "a"}, {Artist: "Другой", Title: "Группа крови", Link: "b"}, {Artist: "Кино", Title: "Звезда", Link: "c"}}
	got := rankSearchResults("группа крови", songs, 20)
	if len(got) != 2 {
		t.Fatalf("lost distinct artists or kept irrelevant results: %+v", got)
	}
	if len(rankSearchResults("Кино", songs, 1)) != 1 {
		t.Fatal("limit ignored")
	}
}

func TestSearchIgnoresAlbumMetadataInArtist(t *testing.T) {
	songs := []domain.Song{{Artist: "Кино", Title: "Группа крови", Link: "a"}, {Artist: "Кино (Группа крови, 1988)", Title: "Закрой за мной дверь", Link: "b"}, {Artist: "Кино Группа крови Label", Title: "Оцифровка", Link: "c"}}
	got := rankSearchResults("кино группа крови", songs, 20)
	if len(got) != 1 || got[0].Link != "a" {
		t.Fatalf("metadata pollution: %+v", got)
	}
}
