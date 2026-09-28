package handlers

import (
	"strings"
	"testing"
)

func TestItunesArtworkSizeUpgrade(t *testing.T) {
	in := "https://is1-ssl.mzstatic.com/image/thumb/Music/v4/e8/x.jpg/100x100bb.jpg"
	got := strings.Replace(in, "100x100", "600x600", 1)
	if !strings.Contains(got, "600x600") || strings.Contains(got, "100x100") {
		t.Fatalf("got %q", got)
	}
}
