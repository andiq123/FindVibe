package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLatin1ToUTF8(t *testing.T) {
	// Google sends ã (0xe3) for Romanian ă
	in := []byte("sandu ciorb\xe3")
	got := string(latin1ToUTF8(in))
	want := "sandu ciorbã"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRoSuggestFix(t *testing.T) {
	if got := roSuggestFix.Replace("sandu ciorbã"); got != "sandu ciorbă" {
		t.Fatalf("got %q", got)
	}
}

func TestLocaleCode(t *testing.T) {
	cases := []struct{ in, fallback, want string }{
		{"en-US", "en", "en"},
		{"RO", "en", "ro"},
		{"", "en", "en"},
		{"x", "en", "en"},
		{"!!", "us", "us"},
	}
	for _, c := range cases {
		if got := localeCode(c.in, c.fallback); got != c.want {
			t.Fatalf("localeCode(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

type suggestTransport func(*http.Request) (*http.Response, error)

func (f suggestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSuggestionsPreserveCyrillic(t *testing.T) {
	client := &http.Client{Transport: suggestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("q") != "кино" || r.URL.Query().Get("oe") != "utf-8" {
			t.Fatalf("bad query: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`["кино",["кино группа крови","кино звезда"]]`))}, nil
	})}
	got, err := NewSuggestionsService(client).GetSuggestions(context.Background(), "кино", "ru", "MD")
	if err != nil || len(got) != 2 || got[0] != "кино группа крови" {
		t.Fatalf("corrupted suggestions: %v %v", got, err)
	}
}
