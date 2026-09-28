package services

import "testing"

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
