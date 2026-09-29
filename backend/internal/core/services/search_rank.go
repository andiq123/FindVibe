package services

import (
	"sort"
	"strings"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

func searchWords(s string) string {
	return strings.Join(strings.Fields(matchPunctuationRe.ReplaceAllString(utils.NormalizeString(s), " ")), " ")
}

// RecordingKey keeps named versions and different artists separate.
func RecordingKey(s domain.Song) string {
	if strings.TrimSpace(s.Artist) == "" || strings.TrimSpace(s.Title) == "" {
		return s.Link
	}
	return searchWords(s.Artist) + "|" + searchWords(s.Title)
}

func sourceRank(provider string) int {
	switch strings.ToLower(provider) {
	case "musicboss":
		return 5
	case "hitmos":
		return 4
	case "musify":
		return 3
	case "mp3mn":
		return 2
	default:
		return 1
	}
}

func rankSearchResults(query string, songs []domain.Song, limit int) []domain.Song {
	// Stable sorting also makes the primary source independent of network arrival order.
	sort.SliceStable(songs, func(i, j int) bool { return sourceRank(songs[i].Provider) > sourceRank(songs[j].Provider) })
	grouped := make([]domain.Song, 0, len(songs))
	keys := make(map[string]int)
	for _, song := range songs {
		key := RecordingKey(song)
		if i, ok := keys[key]; ok {
			if grouped[i].Link != song.Link {
				grouped[i].Alternatives = append(grouped[i].Alternatives, domain.AudioSource{Link: song.Link, Provider: song.Provider})
			}
			if !HasRealCover(grouped[i].Image) && HasRealCover(song.Image) {
				grouped[i].Image = song.Image
			}
		} else {
			keys[key] = len(grouped)
			grouped = append(grouped, song)
		}
	}
	normalizedQuery := searchWords(query)
	tokens := strings.Fields(normalizedQuery)
	exactRecording := false
	for _, song := range grouped {
		artist, title := searchWords(song.Artist), searchWords(song.Title)
		if normalizedQuery == artist+" "+title || normalizedQuery == title+" "+artist || normalizedQuery == title {
			exactRecording = true
			break
		}
	}
	score := func(s domain.Song) int {
		haystack := " " + searchWords(matchParenRe.ReplaceAllString(s.Artist, " ")+" "+s.Title) + " "
		if exactRecording {
			title := " " + searchWords(s.Title) + " "
			matchedTitle := false
			for _, token := range tokens {
				if strings.Contains(title, " "+token+" ") {
					matchedTitle = true
					break
				}
			}
			if !matchedTitle {
				return 0
			}
		}
		n := 0
		for _, token := range tokens {
			if strings.Contains(haystack, " "+token+" ") {
				n++
			}
		}
		return n
	}
	sort.SliceStable(grouped, func(i, j int) bool {
		a, b := score(grouped[i]), score(grouped[j])
		if a != b {
			return a > b
		}
		// Prefer original recordings unless a version was explicitly requested.
		if !matchVersionRe.MatchString(query) {
			aVersion, bVersion := matchVersionRe.MatchString(grouped[i].Title+" "+grouped[i].Artist), matchVersionRe.MatchString(grouped[j].Title+" "+grouped[j].Artist)
			if aVersion != bVersion {
				return !aVersion
			}
		}
		return false
	})
	if len(grouped) > 0 && len(tokens) > 1 && score(grouped[0]) == len(tokens) {
		end := 0
		for end < len(grouped) && score(grouped[end]) == len(tokens) {
			end++
		}
		grouped = grouped[:end]
	}
	if limit > 0 && len(grouped) > limit {
		grouped = grouped[:limit]
	}
	return grouped
}
