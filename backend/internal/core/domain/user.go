package domain

import "github.com/google/uuid"

type User struct {
	ID   string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name string `gorm:"type:varchar(255);not null;uniqueIndex" json:"username"`
}

func NewUser(name string) *User {
	return &User{
		ID:   uuid.New().String(),
		Name: name,
	}
}
