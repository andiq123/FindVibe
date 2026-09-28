package domain

type ProviderResult struct {
	Song         Song
	Provider     string
	ProviderRank int
	Pagination   *PaginationInfo
}

type PaginationInfo struct {
	CurrentPage  int  `json:"currentPage"`
	TotalResults int  `json:"totalResults"`
	HasNextPage  bool `json:"hasNextPage"`
	HasPrevPage  bool `json:"hasPrevPage"`
	TotalPages   int  `json:"totalPages"`
}

func NewProviderResult(song Song, provider string, rank int, pagination *PaginationInfo) ProviderResult {
	song.Provider = provider
	return ProviderResult{
		Song:         song,
		Provider:     provider,
		ProviderRank: rank,
		Pagination:   pagination,
	}
}
