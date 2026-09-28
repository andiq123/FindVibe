package utils

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

// ProxyPool round-robins outbound proxies and cools down ones that get blocked.
// Empty pool → callers should fall back to http.ProxyFromEnvironment / direct.
type ProxyPool struct {
	mu       sync.Mutex
	entries  []*url.URL
	index    int
	cooldown map[string]time.Time
	coolFor  time.Duration
}

func NewProxyPool(entries []*url.URL) *ProxyPool {
	if len(entries) == 0 {
		return nil
	}
	return &ProxyPool{
		entries:  entries,
		cooldown: make(map[string]time.Time, len(entries)),
		coolFor:  2 * time.Minute,
	}
}

// ParseProxyList splits comma / newline separated proxy URLs.
// Supports http://, https://, socks5://, socks5h:// (with optional userinfo).
func ParseProxyList(raw string) []*url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	out := make([]*url.URL, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		u, err := url.Parse(p)
		if err != nil || u.Scheme == "" || u.Host == "" {
			GetLogger().Warn("skipping invalid provider proxy", "value", p)
			continue
		}
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			GetLogger().Warn("skipping unsupported proxy scheme", "scheme", u.Scheme)
			continue
		}
		key := u.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, u)
	}
	return out
}

func (p *ProxyPool) Len() int {
	if p == nil {
		return 0
	}
	return len(p.entries)
}

// Current returns a sticky proxy (skips cooled-down entries). nil pool → nil URL (direct/env).
func (p *ProxyPool) Current() *url.URL {
	if p == nil || len(p.entries) == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pickLocked()
}

// MarkBad cools down the current proxy and advances to the next one.
func (p *ProxyPool) MarkBad() {
	if p == nil || len(p.entries) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cur := p.entries[p.index%len(p.entries)]
	p.cooldown[cur.String()] = time.Now().Add(p.coolFor)
	p.index = (p.index + 1) % len(p.entries)
	GetLogger().Warn("provider proxy cooled down", "proxy", redactedProxy(cur), "coolFor", p.coolFor.String())
}

func (p *ProxyPool) pickLocked() *url.URL {
	n := len(p.entries)
	now := time.Now()
	for i := 0; i < n; i++ {
		idx := (p.index + i) % n
		u := p.entries[idx]
		if until, ok := p.cooldown[u.String()]; ok && now.Before(until) {
			continue
		}
		p.index = idx
		return u
	}
	// All cooling — pick the one that frees soonest.
	best := p.entries[p.index%n]
	bestUntil := p.cooldown[best.String()]
	for _, u := range p.entries {
		if until := p.cooldown[u.String()]; until.Before(bestUntil) {
			best = u
			bestUntil = until
		}
	}
	p.index = indexOfURL(p.entries, best)
	return best
}

func indexOfURL(list []*url.URL, want *url.URL) int {
	for i, u := range list {
		if u.String() == want.String() {
			return i
		}
	}
	return 0
}

func redactedProxy(u *url.URL) string {
	if u == nil {
		return ""
	}
	c := *u
	if c.User != nil {
		if _, has := c.User.Password(); has {
			c.User = url.UserPassword(c.User.Username(), "***")
		} else {
			c.User = url.User(c.User.Username())
		}
	}
	return c.String()
}
