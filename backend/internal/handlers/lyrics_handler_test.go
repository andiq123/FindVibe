package handlers

import "testing"

func TestPickLyricsPrefersPlain(t *testing.T) {
	got := pickLyrics([]lrclibHit{
		{Instrumental: true, PlainLyrics: "nope"},
		{SyncedLyrics: "[00:01.00] Hello\n[00:02.00] World"},
		{PlainLyrics: "Plain text"},
	})
	if got != "Plain text" {
		t.Fatalf("got %q", got)
	}
}

func TestPickLyricsStripsSyncedStamps(t *testing.T) {
	got := pickLyrics([]lrclibHit{
		{SyncedLyrics: "[00:01.00] Hello\n[00:02.00] World\n"},
	})
	if got != "Hello\nWorld" {
		t.Fatalf("got %q", got)
	}
}

func TestAllInstrumental(t *testing.T) {
	if !allInstrumental([]lrclibHit{{Instrumental: true}, {Instrumental: true}}) {
		t.Fatal("want true")
	}
	if allInstrumental(nil) || allInstrumental([]lrclibHit{{Instrumental: false}}) {
		t.Fatal("want false")
	}
}

func TestCleanLyricsQueryStripsSiteTags(t *testing.T) {
	got := cleanLyricsQuery("Irina Rimes [mp3-you.net]")
	if got != "Irina Rimes" {
		t.Fatalf("got %q", got)
	}
	got = cleanLyricsQuery("S-a întâmplat Cu Noi (Official Audio)")
	if got != "S-a întâmplat Cu Noi" {
		t.Fatalf("got %q", got)
	}
}

func TestLyricsCacheHitAndMissTTL(t *testing.T) {
	h := NewLyricsHandler(nil)
	key := lyricsCacheKey("Artist", "Title [Official Audio]")
	h.cachePut(key, "hello", "")
	text, code, ok := h.cacheGet(key)
	if !ok || text != "hello" || code != "" {
		t.Fatalf("hit: ok=%v text=%q code=%q", ok, text, code)
	}
	h.cachePut(key, "", "not_found")
	_, code, ok = h.cacheGet(key)
	if !ok || code != "not_found" {
		t.Fatalf("miss: ok=%v code=%q", ok, code)
	}
}
