package providers

import (
	"testing"
)

func TestIsBlockedBody(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{`<div id="results" class="item">`, false},
		{`<html>Just a moment...</html>`, true},
		{`cf-browser-verification`, true},
		{`DDoS-Guard`, true},
		{``, false},
	}
	for _, tc := range cases {
		if got := isBlockedBody([]byte(tc.body)); got != tc.want {
			t.Fatalf("body %q: got %v want %v", tc.body, got, tc.want)
		}
	}
}

func TestIsBlockedStatus(t *testing.T) {
	if !isBlockedStatus(403) || !isBlockedStatus(429) || isBlockedStatus(200) || isBlockedStatus(404) {
		t.Fatal("status mapping wrong")
	}
}

func TestIsNotFoundAndBlockedErr(t *testing.T) {
	if !isNotFoundStatus(errString("Mp3pm: status 404")) {
		t.Fatal("404")
	}
	if !isBlockedErr(errString("Musify: status 403 (blocked)")) {
		t.Fatal("403 blocked")
	}
	if isBlockedErr(errString("Mp3pm: status 404")) || isNotFoundStatus(nil) {
		t.Fatal("cross-check")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
