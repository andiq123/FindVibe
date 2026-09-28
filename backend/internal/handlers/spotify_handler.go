package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Embed page ships ~50 tracks in __NEXT_DATA__ — no Spotify app / secrets.
const spotifyTrackCap = 50

var (
	spotifyPlaylistIDRe = regexp.MustCompile(`(?i)(?:playlist[/:]|spotify:playlist:)([a-zA-Z0-9]{22})`)
	spotifyBareIDRe     = regexp.MustCompile(`^[a-zA-Z0-9]{22}$`)
	spotifyNextDataRe   = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)
)

type SpotifyHandler struct {
	client *http.Client
}

func NewSpotifyHandler(client *http.Client) *SpotifyHandler {
	return &SpotifyHandler{client: client}
}

type SpotifyTrackMeta struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
}

type SpotifyPlaylistResponse struct {
	Name   string             `json:"name"`
	Tracks []SpotifyTrackMeta `json:"tracks"`
}

// GET /spotify/playlist?url= — public playlist via Spotify embed (no API keys).
func (h *SpotifyHandler) GetPlaylist(c fiber.Ctx) error {
	id := ParseSpotifyPlaylistID(c.Query("url"))
	if id == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Paste a public Spotify playlist link",
		})
	}

	name, tracks, status, err := h.fetchEmbedPlaylist(c.Context(), id)
	if err != nil {
		if status == http.StatusNotFound || status == http.StatusForbidden {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"error": "Playlist not found or not public",
			})
		}
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't load Spotify playlist"})
	}
	if len(tracks) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Playlist has no tracks"})
	}
	return c.JSON(SpotifyPlaylistResponse{Name: name, Tracks: tracks})
}

// ParseSpotifyPlaylistID extracts a 22-char playlist id from URL, URI, or bare id.
func ParseSpotifyPlaylistID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := spotifyPlaylistIDRe.FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	if spotifyBareIDRe.MatchString(raw) {
		return raw
	}
	return ""
}

func (h *SpotifyHandler) fetchEmbedPlaylist(ctx context.Context, id string) (string, []SpotifyTrackMeta, int, error) {
	u := "https://open.spotify.com/embed/playlist/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FindVibe/1.0)")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, resp.StatusCode, fmt.Errorf("spotify embed %d", resp.StatusCode)
	}

	name, tracks, err := ParseSpotifyEmbedHTML(string(body))
	if err != nil {
		return "", nil, 0, err
	}
	return name, tracks, http.StatusOK, nil
}

// ParseSpotifyEmbedHTML reads playlist name + tracks from embed __NEXT_DATA__.
func ParseSpotifyEmbedHTML(html string) (string, []SpotifyTrackMeta, error) {
	m := spotifyNextDataRe.FindStringSubmatch(html)
	if len(m) != 2 {
		return "", nil, fmt.Errorf("embed data missing")
	}

	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							Type      string `json:"type"`
							Name      string `json:"name"`
							Title     string `json:"title"`
							TrackList []struct {
								Title      string `json:"title"`
								Subtitle   string `json:"subtitle"`
								EntityType string `json:"entityType"`
							} `json:"trackList"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return "", nil, err
	}

	ent := data.Props.PageProps.State.Data.Entity
	if !strings.EqualFold(ent.Type, "playlist") {
		return "", nil, fmt.Errorf("not a playlist")
	}
	name := strings.TrimSpace(ent.Name)
	if name == "" {
		name = strings.TrimSpace(ent.Title)
	}
	if name == "" {
		name = "Spotify playlist"
	}

	tracks := make([]SpotifyTrackMeta, 0, len(ent.TrackList))
	for _, t := range ent.TrackList {
		if len(tracks) >= spotifyTrackCap {
			break
		}
		if t.EntityType != "" && !strings.EqualFold(t.EntityType, "track") {
			continue
		}
		title := strings.TrimSpace(t.Title)
		artist := strings.TrimSpace(t.Subtitle)
		// "Artist1, Artist2" → first artist is enough for search
		if i := strings.Index(artist, ","); i > 0 {
			artist = strings.TrimSpace(artist[:i])
		}
		if title == "" || artist == "" {
			continue
		}
		tracks = append(tracks, SpotifyTrackMeta{Artist: artist, Title: title})
	}
	return name, tracks, nil
}
