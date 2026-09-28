package services

import "testing"

func TestCoreTitleStripsRemixVariants(t *testing.T) {
	a := CoreTitle("Collide (Extended Mix)")
	b := CoreTitle("Collide feat Rosi Golan [Extended Mix]")
	c := CoreTitle("Collide (Original Mix)")
	if a != "collide" || b != "collide" || c != "collide" {
		t.Fatalf("got %q %q %q", a, b, c)
	}
}

func TestSongKeyStable(t *testing.T) {
	if SongKey("Vicetone", "Collide (Extended Mix)") != SongKey("vicetone", "Collide") {
		t.Fatal("song keys should collapse remix variants")
	}
}
