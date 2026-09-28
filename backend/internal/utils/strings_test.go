package utils

import "testing"

func TestUpgradeHTTPS(t *testing.T) {
	cases := map[string]string{
		"http://a/x":  "https://a/x",
		"//cdn/a.jpg": "https://cdn/a.jpg",
		"https://ok":  "https://ok",
		"  http://z ": "https://z",
		"":            "",
	}
	for in, want := range cases {
		if got := UpgradeHTTPS(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
