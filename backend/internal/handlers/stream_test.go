package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type streamTransport func(*http.Request) (*http.Response, error)

func (f streamTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type contextBody struct {
	ctx context.Context
	io.Reader
	closed bool
}

func (b *contextBody) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.Reader.Read(p)
}
func (b *contextBody) Close() error { b.closed = true; return nil }

func TestStreamSurvivesHandlerReturnAndPreservesRange(t *testing.T) {
	payload := strings.Repeat("audio", 16000)
	var body *contextBody
	client := &http.Client{Transport: streamTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Range") != "bytes=100-" {
			t.Error("range not forwarded")
		}
		body = &contextBody{ctx: r.Context(), Reader: strings.NewReader(payload)}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Type": {"audio/mpeg"}, "Content-Range": {"bytes 100-80099/80100"}}, ContentLength: int64(len(payload)), Body: body}, nil
	})}
	h := NewRecommendHandlerUpstream(client, client, "", stubSearch{}, nil)
	app := fiber.New()
	app.Get("/stream", h.GetStream)
	req := httptest.NewRequest("GET", "/stream?artist=Test&title=Song&link=https%3A%2F%2Fmp3.pm%2Fsong.mp3", nil)
	req.Header.Set("Range", "bytes=100-")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 206 || string(got) != payload || resp.Header.Get("Content-Range") != "bytes 100-80099/80100" {
		t.Fatalf("incomplete stream: status %d, bytes %d", resp.StatusCode, len(got))
	}
	if !body.closed || body.ctx.Err() == nil {
		t.Fatal("upstream body/context leaked")
	}
}
func TestStreamRejectsHTMLAndUnsafeHosts(t *testing.T) {
	for _, raw := range []string{"https://evilsunproxy.net/a", "https://sunproxy.net.evil.test/a", "https://user@mp3.pm/a", "https://mp3.pm:8443/a", "http://mp3.pm/a"} {
		u, _ := url.Parse(raw)
		if streamProxyAllowed(u) {
			t.Errorf("allowed %s", raw)
		}
	}
	if validAudioResponse(&http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}}) {
		t.Fatal("HTML accepted as audio")
	}
	if validAudioResponse(&http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/mpeg"}}, ContentLength: 81 << 20}) {
		t.Fatal("oversized audio accepted")
	}
}
func TestStreamRejectsRedirectOutsideProviders(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: streamTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"http://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	h := NewRecommendHandlerUpstream(client, client, "", stubSearch{}, nil)
	resp, err := h.openStreamUpstream(context.Background(), "https://mp3.pm/song.mp3", "")
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || calls != 1 {
		t.Fatalf("redirect followed: calls=%d err=%v", calls, err)
	}
}
