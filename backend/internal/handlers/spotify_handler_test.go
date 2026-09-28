package handlers

import "testing"

func TestParseSpotifyPlaylistID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", "37i9dQZF1DXcBWIGoYBM5M"},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=abc", "37i9dQZF1DXcBWIGoYBM5M"},
		{"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", "37i9dQZF1DXcBWIGoYBM5M"},
		{"37i9dQZF1DXcBWIGoYBM5M", "37i9dQZF1DXcBWIGoYBM5M"},
		{"https://open.spotify.com/track/abc", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ParseSpotifyPlaylistID(tc.in); got != tc.want {
			t.Fatalf("ParseSpotifyPlaylistID(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseSpotifyEmbedHTML(t *testing.T) {
	html := `<html><script id="__NEXT_DATA__" type="application/json">` +
		`{"props":{"pageProps":{"state":{"data":{"entity":{` +
		`"type":"playlist","name":"Test Mix","trackList":[` +
		`{"title":"Song A","subtitle":"Artist One","entityType":"track"},` +
		`{"title":"Song B","subtitle":"Artist Two, Feat","entityType":"track"},` +
		`{"title":"Episode","subtitle":"Host","entityType":"episode"}` +
		`]}}}}}}` +
		`</script></html>`
	name, tracks, err := ParseSpotifyEmbedHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Test Mix" {
		t.Fatalf("name=%q", name)
	}
	if len(tracks) != 2 {
		t.Fatalf("tracks=%d want 2", len(tracks))
	}
	if tracks[0].Artist != "Artist One" || tracks[0].Title != "Song A" {
		t.Fatalf("track0=%+v", tracks[0])
	}
	if tracks[1].Artist != "Artist Two" {
		t.Fatalf("multi-artist first only, got %q", tracks[1].Artist)
	}
}
