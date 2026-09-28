package utils

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ScrapeClient is an http.Client tuned for HTML providers (cookies, env/list proxies, rotate on block).
type ScrapeClient struct {
	*http.Client
	Pool *ProxyPool
	rt   *rotatingTransport
}

// NewScrapeClient builds a jar-enabled client. proxies may be empty (still honors HTTP(S)_PROXY).
func NewScrapeClient(timeout time.Duration, maxIdleConns, maxIdlePerHost int, idleTimeout time.Duration, proxies []*url.URL) *ScrapeClient {
	pool := NewProxyPool(proxies)
	rt := newRotatingTransport(pool, maxIdleConns, maxIdlePerHost, idleTimeout)
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: rt,
		Jar:       jar,
	}
	if pool != nil {
		GetLogger().Info("provider proxy pool ready", "count", pool.Len())
	}
	return &ScrapeClient{Client: client, Pool: pool, rt: rt}
}

// Rotate moves to the next proxy after a block / hard failure.
func (c *ScrapeClient) Rotate() {
	if c == nil {
		return
	}
	hadProxy := c.Pool != nil && c.Pool.Len() > 0
	if c.Pool != nil {
		c.Pool.MarkBad()
	}
	if c.rt != nil {
		c.rt.invalidateSticky()
	}
	// Cookies are IP/session-bound — only drop them when egress actually changes.
	// Clearing the jar on a direct-only client makes 403 loops worse (loses _vt etc.).
	if hadProxy {
		if jar, err := cookiejar.New(nil); err == nil {
			c.Jar = jar
		}
	}
}

// AttemptBudget how many fetch tries (direct/env + each proxy, capped).
func (c *ScrapeClient) AttemptBudget(fallback int) int {
	if fallback < 1 {
		fallback = 1
	}
	if c == nil || c.Pool == nil {
		return fallback
	}
	n := 1 + c.Pool.Len()
	if n > 4 {
		n = 4
	}
	if n < fallback {
		return fallback
	}
	return n
}

type rotatingTransport struct {
	pool           *ProxyPool
	maxIdleConns   int
	maxIdlePerHost int
	idleTimeout    time.Duration

	mu     sync.Mutex
	sticky string
	cached map[string]*http.Transport
	direct *http.Transport
}

func newRotatingTransport(pool *ProxyPool, maxIdleConns, maxIdlePerHost int, idleTimeout time.Duration) *rotatingTransport {
	return &rotatingTransport{
		pool:           pool,
		maxIdleConns:   maxIdleConns,
		maxIdlePerHost: maxIdlePerHost,
		idleTimeout:    idleTimeout,
		cached:         make(map[string]*http.Transport),
		direct:         buildTransport(nil, maxIdleConns, maxIdlePerHost, idleTimeout),
	}
}

func (rt *rotatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := rt.pickTransport()
	return tr.RoundTrip(req)
}

func (rt *rotatingTransport) invalidateSticky() {
	rt.mu.Lock()
	rt.sticky = ""
	rt.mu.Unlock()
}

func (rt *rotatingTransport) pickTransport() *http.Transport {
	if rt.pool == nil || rt.pool.Len() == 0 {
		return rt.direct
	}
	u := rt.pool.Current()
	if u == nil {
		return rt.direct
	}
	key := u.String()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.sticky == "" {
		rt.sticky = key
	}
	// Prefer sticky until Rotate clears it.
	if t := rt.cached[rt.sticky]; t != nil {
		return t
	}
	if t := rt.cached[key]; t != nil {
		rt.sticky = key
		return t
	}
	t := buildTransport(u, rt.maxIdleConns, rt.maxIdlePerHost, rt.idleTimeout)
	rt.cached[key] = t
	rt.sticky = key
	return t
}

func buildTransport(proxyURL *url.URL, maxIdleConns, maxIdlePerHost int, idleTimeout time.Duration) *http.Transport {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdlePerHost,
		IdleConnTimeout:       idleTimeout,
		ResponseHeaderTimeout: 10 * time.Second,
		// HTTP/2 fingerprints are a common scrape block signal.
		ForceAttemptHTTP2: false,
		Proxy:             http.ProxyFromEnvironment,
	}
	if proxyURL == nil {
		return tr
	}

	scheme := proxyURL.Scheme
	switch scheme {
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			GetLogger().Warn("socks proxy dialer failed, falling back to env/direct", "error", err)
			return tr
		}
		tr.Proxy = nil
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return cd.DialContext(ctx, network, addr)
			}
		} else {
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	default:
		tr.Proxy = http.ProxyURL(proxyURL)
	}
	return tr
}
