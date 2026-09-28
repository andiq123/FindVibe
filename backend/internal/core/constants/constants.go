package constants

const (
	// HTTP Client Configuration
	DefaultHTTPTimeout        = 12 // seconds — scrape + redirect from Render needs headroom
	DefaultHTTPMaxIdleConns   = 100
	DefaultHTTPMaxIdlePerHost = 10
	DefaultHTTPIdleTimeout    = 90 // seconds

	// Database Configuration
	DefaultDBPort              = 5432
	DefaultDBMaxOpenConns      = 25
	DefaultDBMaxIdleConns      = 10
	DefaultDBConnMaxLifetime   = 5  // minutes
	DefaultDBConnMaxIdleTime   = 10 // minutes

	// Search Service Configuration
	// Proxies + homepage warm-up need a bit more than a direct scrape.
	DefaultSearchTimeout    = 5 // seconds
	DefaultMaxSearchResults = 20
	DefaultMaxPageNumber    = 100
	MaxQueryLength          = 200
	MinQueryLength          = 1

	// Concurrent Last.fm → /search resolves (explore/radio). Unbounded floods providers.
	DefaultResolveConcurrency = 6

	// Server Configuration
	DefaultServerPort  = "8080"
	DefaultReadTimeout = 15 // seconds
	// Explore stream can emit shelves for ~55s; write timeout must cover the full NDJSON body.
	DefaultWriteTimeout = 60 // seconds
	DefaultIdleTimeout  = 120 // seconds

	// Request Limits
	MaxRequestSize = 1024 * 1024 // 1MB

	// Validation
	MinUsernameLength = 1
	MaxUsernameLength = 100
)
