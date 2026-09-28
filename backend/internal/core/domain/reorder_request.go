package domain

import (
	"encoding/json"
	"strings"
)

// ReorderRequest accepts both Angular (`songId`) and iOS (`id`) JSON keys.
type ReorderRequest struct {
	SongId string `json:"songId"`
	Order  int    `json:"order"`
}

func (r *ReorderRequest) UnmarshalJSON(data []byte) error {
	var aux struct {
		SongId string `json:"songId"`
		ID     string `json:"id"`
		Order  int    `json:"order"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.SongId = strings.TrimSpace(aux.SongId)
	if r.SongId == "" {
		r.SongId = strings.TrimSpace(aux.ID)
	}
	r.Order = aux.Order
	return nil
}
