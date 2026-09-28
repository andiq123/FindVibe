package domain

import (
	"encoding/json"
	"testing"
)

func TestReorderRequestAcceptsSongIdOrID(t *testing.T) {
	var a ReorderRequest
	if err := json.Unmarshal([]byte(`{"songId":"abc","order":2}`), &a); err != nil {
		t.Fatal(err)
	}
	if a.SongId != "abc" || a.Order != 2 {
		t.Fatalf("songId path: %#v", a)
	}

	var b ReorderRequest
	if err := json.Unmarshal([]byte(`{"id":"xyz","order":1}`), &b); err != nil {
		t.Fatal(err)
	}
	if b.SongId != "xyz" || b.Order != 1 {
		t.Fatalf("id path: %#v", b)
	}
}
