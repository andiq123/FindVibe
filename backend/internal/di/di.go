package di

import (
	"os"

	"github.com/andiq123/FindVibeFiber/internal/config"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"github.com/andiq123/FindVibeFiber/internal/core/services"
	"github.com/andiq123/FindVibeFiber/internal/core/services/providers"
	"github.com/andiq123/FindVibeFiber/internal/handlers"
	"github.com/andiq123/FindVibeFiber/internal/repository"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"gorm.io/gorm"
)

type Handlers struct {
	Health      *handlers.HealthHandler
	Auth        *handlers.AuthHandler
	Favorites   *handlers.FavoritesHandler
	Suggestions *handlers.SuggestionsHandler
	Search      *handlers.SearchHandler
	Cover       *handlers.CoverHandler
	Recommend   *handlers.RecommendHandler
	Lyrics      *handlers.LyricsHandler
	Spotify     *handlers.SpotifyHandler
}

func InitializeHandlers(db *gorm.DB, cfg *config.AppConfig) Handlers {
	authRepository := repository.NewAuthRepository(db)
	favoritesRepository := repository.NewFavoritesRepository(db)

	httpClient := utils.NewHTTPClient(
		cfg.HTTP.Timeout,
		cfg.HTTP.MaxIdleConns,
		cfg.HTTP.MaxIdlePerHost,
		cfg.HTTP.IdleTimeout,
	)

	// Dedicated scrape client: cookie jar, HTTP/1.1, PROXY env + PROVIDER_PROXIES rotation.
	scrape := utils.NewScrapeClient(
		cfg.HTTP.Timeout,
		cfg.HTTP.MaxIdleConns,
		cfg.HTTP.MaxIdlePerHost,
		cfg.HTTP.IdleTimeout,
		utils.ParseProxyList(cfg.HTTP.ProviderProxies),
	)
	mp3pm := providers.NewMp3pmProvider(scrape.Client).UseRotator(scrape)
	mp3mn := providers.NewMp3mnProvider(scrape.Client)
	mp3mn.WithRotator(scrape)
	musify := providers.NewMusifyProvider(scrape.Client).UseRotator(scrape)

	searchConfig := domain.DefaultSearchConfig()
	searchConfig.MaxResults = cfg.Search.MaxResults

	lastfmKey := os.Getenv("LASTFM_API_KEY")
	covers := services.NewCoverService(httpClient, lastfmKey)
	catalog := services.NewLastFMCatalog(httpClient, lastfmKey)
	searchSvc := services.NewSearchService(
		[]ports.IMusicProvider{mp3pm, mp3mn, musify},
		searchConfig,
		cfg.Search.Timeout,
		catalog,
	)
	searchSvc.SetCovers(covers)

	return Handlers{
		Health:      handlers.NewHealthHandler(scrape.Client),
		Auth:        handlers.NewAuthHandler(services.NewAuthService(authRepository)),
		Favorites:   handlers.NewFavoritesHandler(services.NewFavoritesService(favoritesRepository, authRepository)),
		Suggestions: handlers.NewSuggestionsHandler(services.NewSuggestionsService(httpClient)),
		Cover:       handlers.NewCoverHandler(covers),
		Search:      handlers.NewSearchHandler(searchSvc, covers),
		Recommend:   handlers.NewRecommendHandlerUpstream(httpClient, scrape.Client, lastfmKey, searchSvc, covers),
		Lyrics:      handlers.NewLyricsHandler(httpClient),
		Spotify:     handlers.NewSpotifyHandler(httpClient),
	}
}
