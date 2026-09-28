package utils

import (
	"testing"
)

func TestParseProxyList(t *testing.T) {
	got := ParseProxyList("http://a:1@host:8000, socks5://host:1080\nhttps://b:2@other:443, bogus, ftp://x")
	if len(got) != 3 {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	if got[0].Scheme != "http" || got[1].Scheme != "socks5" || got[2].Scheme != "https" {
		t.Fatalf("schemes: %v", got)
	}
}

func TestProxyPoolRotate(t *testing.T) {
	urls := ParseProxyList("http://p1:1,http://p2:2")
	pool := NewProxyPool(urls)
	if pool.Len() != 2 {
		t.Fatal(pool.Len())
	}
	a := pool.Current().Host
	pool.MarkBad()
	b := pool.Current().Host
	if a == b {
		t.Fatalf("expected rotation after MarkBad, both %q", a)
	}
}

func TestProxyPoolNil(t *testing.T) {
	var pool *ProxyPool
	if pool.Len() != 0 || pool.Current() != nil {
		t.Fatal("nil pool should be empty")
	}
	pool.MarkBad() // must not panic
}
