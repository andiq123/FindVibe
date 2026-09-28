package domain

type SearchConfig struct {
	MaxResults int
}

func DefaultSearchConfig() *SearchConfig {
	return &SearchConfig{
		MaxResults: 20,
	}
}
