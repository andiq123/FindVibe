package domain

// SearchArtist is a Last.fm artist.match for discovery UI (not a stream).
type SearchArtist struct {
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
}

type SearchResponse struct {
	Songs      []Song          `json:"songs"`
	Artists    []SearchArtist  `json:"artists,omitempty"`
	Albums     []ArtistAlbum   `json:"albums,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// SearchProgress is discovery chrome emitted before / while songs map (stream path).
type SearchProgress struct {
	Artists    []SearchArtist
	Albums     []ArtistAlbum
	Pagination *PaginationInfo
}

func NewSearchResponse(songs []Song, pagination *PaginationInfo) *SearchResponse {
	return &SearchResponse{
		Songs:      songs,
		Pagination: pagination,
	}
}
