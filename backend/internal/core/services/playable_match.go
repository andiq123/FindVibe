package services

import (
	"regexp"
	"strings"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

var (
	matchParenRe = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	matchFeatRe  = regexp.MustCompile(`(?i)\s*(feat\.?|ft\.?|featuring)\s+.*$`)
	matchJunkRe  = regexp.MustCompile(`(?i)\b(original\s+mix|extended\s+mix|radio\s+edit|club\s+mix|remix|bootleg|edit|mix|version|remaster(ed)?|instrumental|karaoke|live|acoustic|dub)\b`)
	matchSpaceRe = regexp.MustCompile(`\s+`)
)

// SongKey collapses remix/feat variants: "Collide (Extended Mix)" ≈ "Collide".
func SongKey(artist, title string) string {
	a, t := utils.NormalizeString(artist), CoreTitle(title)
	if a == "" || t == "" {
		return ""
	}
	return a + "|" + t
}

func CoreTitle(title string) string {
	s := utils.NormalizeString(title)
	s = matchParenRe.ReplaceAllString(s, " ")
	s = matchFeatRe.ReplaceAllString(s, " ")
	s = matchJunkRe.ReplaceAllString(s, " ")
	s = matchSpaceRe.ReplaceAllString(strings.TrimSpace(s), " ")
	return s
}

// IsPlayableMatch: https stream + artist/title overlap with the catalog want.
// Short cores must match exactly — "me"/"up" must not latch via Contains.
func IsPlayableMatch(wantArtist, wantTitle string, got domain.Song) bool {
	if !strings.HasPrefix(strings.TrimSpace(got.Link), "https://") {
		return false
	}
	gotArtist := strings.TrimSpace(got.Artist)
	gotTitle := strings.TrimSpace(got.Title)
	if gotArtist == "" || gotTitle == "" {
		return false
	}

	wantKey := SongKey(wantArtist, wantTitle)
	gotKey := SongKey(gotArtist, gotTitle)
	if wantKey != "" && wantKey == gotKey {
		return true
	}

	wantArt := utils.NormalizeString(wantArtist)
	gotArt := utils.NormalizeString(gotArtist)
	wantCore := CoreTitle(wantTitle)
	gotCore := CoreTitle(gotTitle)
	if wantArt == "" || gotArt == "" || wantCore == "" || gotCore == "" {
		return false
	}
	return artsOverlap(wantArt, gotArt) && titleCoresOverlap(wantCore, gotCore)
}

func artsOverlap(want, got string) bool {
	if want == got {
		return true
	}
	return strings.Contains(got, want) || strings.Contains(want, got)
}

func titleCoresOverlap(want, got string) bool {
	if want == got {
		return true
	}
	if len(want) < 4 || len(got) < 4 {
		return false
	}
	return strings.Contains(got, want) || strings.Contains(want, got)
}

// PickPlayableSong chooses the strongest artist+title match in a provider peek.
func PickPlayableSong(wantArtist, wantTitle string, songs []domain.Song, seedKey string, peek int) (domain.Song, bool) {
	wantKey := SongKey(wantArtist, wantTitle)
	wantCore := CoreTitle(wantTitle)
	wantArt := utils.NormalizeString(wantArtist)

	bestRank := -1
	var best domain.Song
	n := peek
	if n <= 0 || n > len(songs) {
		n = len(songs)
	}
	for i := 0; i < n; i++ {
		s := songs[i]
		gotKey := SongKey(s.Artist, s.Title)
		if gotKey != "" && gotKey == seedKey {
			continue
		}
		if !IsPlayableMatch(wantArtist, wantTitle, s) {
			continue
		}
		rank := 0
		if wantKey != "" && gotKey == wantKey {
			rank += 8
		}
		gotCore := CoreTitle(s.Title)
		gotArt := utils.NormalizeString(s.Artist)
		if gotCore == wantCore {
			rank += 4
		}
		if gotArt == wantArt {
			rank += 2
		}
		if !IsRemixy(s.Title) {
			rank += 1
		}
		if rank > bestRank {
			bestRank = rank
			best = s
		}
	}
	if bestRank < 0 {
		return domain.Song{}, false
	}
	return best, true
}

func IsRemixy(title string) bool {
	t := utils.NormalizeString(title)
	return matchJunkRe.MatchString(t) || matchParenRe.MatchString(t)
}
