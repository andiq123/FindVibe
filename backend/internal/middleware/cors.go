package middleware

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3/middleware/cors"
)

// NewCORS — browser clients (Angular / WebView). Native iOS URLSession ignores CORS.
func NewCORS() cors.Config {
	origins := []string{
		"https://find-vibe.vercel.app",
		"http://localhost:4200",
		"http://127.0.0.1:4200",
	}
	if extra := os.Getenv("CORS_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				origins = append(origins, o)
			}
		}
	}

	cfg := cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "ngrok-skip-browser-warning", "X-Requested-With"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Length"},
		MaxAge:           86400, // cache preflight — fewer OPTIONS round-trips
	}

	// Local / tunnel / LAN origins for debug WebViews and Angular on device.
	if os.Getenv("IS_LOCAL") == "true" {
		cfg.AllowOriginsFunc = allowDevOrigin
	}
	return cfg
}

func allowDevOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return u.Scheme == "http" || u.Scheme == "https"
	}
	// Cloudflare quick tunnels used with local API debugging.
	if strings.HasSuffix(host, ".trycloudflare.com") {
		return u.Scheme == "https"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return u.Scheme == "http" && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast())
}
