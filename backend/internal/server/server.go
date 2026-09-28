package server

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/config"
	"github.com/andiq123/FindVibeFiber/internal/core/constants"
	"github.com/andiq123/FindVibeFiber/internal/di"
	"github.com/andiq123/FindVibeFiber/internal/middleware"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

type Server struct {
	app *fiber.App
	cfg config.ServerConfig
	h   atomic.Pointer[di.Handlers]
}

// NewServer listens with /health immediately; call Mount after DB is ready.
func NewServer(cfg config.ServerConfig) *Server {
	s := &Server{cfg: cfg}
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		BodyLimit:    constants.MaxRequestSize,
		// Render sits behind a proxy; trust private/link-local so c.IP() is the client
		TrustProxy:       true,
		ProxyHeader:      fiber.HeaderXForwardedFor,
		TrustProxyConfig: fiber.TrustProxyConfig{Private: true, Loopback: true, LinkLocal: true},
	})

	app.Use(cors.New(middleware.NewCORS()))
	app.Use(recover.New())
	// Skip compress on NDJSON streams — buffering would defeat progressive emit.
	app.Use(compress.New(compress.Config{
		Next: func(c fiber.Ctx) bool {
			path := c.Path()
			if path == "/stream" {
				return true
			}
			stream := c.Query("stream") == "1" || strings.EqualFold(c.Query("stream"), "true")
			return stream && (path == "/explore" || path == "/search")
		},
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        180,
		Expiration: time.Minute,
		Next: func(c fiber.Ctx) bool {
			path := c.Path()
			// Health + high-churn mobile paths (vault cover backfill, typeahead).
			return path == "/health" ||
				strings.HasPrefix(path, "/health/") ||
				path == "/cover" ||
				path == "/suggest"
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
		},
	}))

	// Always answer — Render free tier 504s until something listens.
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON("Pong")
	})
	app.Get("/health/sources", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Health.GetSources(c)
	}))
	app.Get("/suggest", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Suggestions.GetSuggestions(c)
	}))
	app.Get("/cover", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Cover.GetCover(c)
	}))
	app.Get("/recommend", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetRecommend(c)
	}))
	app.Get("/similar-artists", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetSimilarArtists(c)
	}))
	app.Get("/artist-albums", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetArtistAlbums(c)
	}))
	app.Get("/album-tracks", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetAlbumTracks(c)
	}))
	app.Get("/explore/cache", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetExploreCache(c)
	}))
	app.Get("/explore", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetExplore(c)
	}))
	app.Get("/lyrics", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Lyrics.GetLyrics(c)
	}))
	app.Get("/search", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Search.Search(c)
	}))
	app.Get("/resolve", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetResolve(c)
	}))
	app.Get("/stream", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Recommend.GetStream(c)
	}))
	app.Get("/spotify/playlist", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Spotify.GetPlaylist(c)
	}))

	favorites := app.Group("/favorites")
	favorites.Get("/:userId", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.GetFavorites(c)
	}))
	favorites.Post("/:userId", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.AddFavorite(c)
	}))
	favorites.Patch("/:songId/image", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.UpdateFavoriteImage(c)
	}))
	favorites.Patch("/:songId/lyrics", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.UpdateFavoriteLyrics(c)
	}))
	favorites.Patch("/:songId/link", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.UpdateFavoriteLink(c)
	}))
	favorites.Delete("/:songId", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.DeleteFavorite(c)
	}))
	favorites.Put("/", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Favorites.ReorderFavorites(c)
	}))

	app.Get("/:username", s.withHandlers(func(h *di.Handlers, c fiber.Ctx) error {
		return h.Auth.AuthenticateUser(c)
	}))

	s.app = app
	return s
}

func (s *Server) Mount(h di.Handlers) {
	s.h.Store(&h)
	utils.GetLogger().Info("API handlers mounted")
}

func (s *Server) withHandlers(fn func(*di.Handlers, fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		h := s.h.Load()
		if h == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "starting"})
		}
		return fn(h, c)
	}
}

func (s *Server) Start() {
	utils.GetLogger().Info("Server starting", "port", s.cfg.Port)
	if err := s.app.Listen(fmt.Sprintf(":%s", s.cfg.Port)); err != nil {
		utils.GetLogger().Error("Server failed to start", "error", err)
		panic(err)
	}
}
