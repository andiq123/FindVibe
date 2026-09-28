package domain

import "github.com/google/uuid"

type Song struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Image    string `json:"image"`
	Link     string `json:"link"`
	Provider string `json:"provider,omitempty"`
}

func NewSong(title string, artist string, image string, link string) *Song {
	return &Song{
		Id:     uuid.New().String(),
		Title:  title,
		Artist: artist,
		Image:  image,
		Link:   link,
	}
}
