package domain

// ArtistAlbum is a cleaned Last.fm top-album row (no stream URL — browse only).
type ArtistAlbum struct {
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	Image     string `json:"image,omitempty"`
	Playcount int64  `json:"playcount,omitempty"`
}
