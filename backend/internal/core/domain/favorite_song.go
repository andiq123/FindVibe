package domain

import "time"

type FavoriteSong struct {
	ID        string    `gorm:"primaryKey;type:varchar(255);index:idx_id_user,priority:1" json:"id"`
	Title     string    `gorm:"type:varchar(500);not null" json:"title"`
	Artist    string    `gorm:"type:varchar(500);not null" json:"artist"`
	Image     string    `gorm:"type:varchar(1000)" json:"image"`
	Lyrics    string    `gorm:"type:text" json:"lyrics,omitempty"`
	Link      string    `gorm:"type:varchar(1000)" json:"link"`
	Order     int       `gorm:"not null;default:0;index:idx_user_order,priority:2" json:"order"`
	UserID    string    `gorm:"column:user_uuid;type:varchar(255);not null;index:,priority:1;index:idx_id_user,priority:2;index:idx_user_order,priority:1" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (FavoriteSong) TableName() string {
	return "favorite_songs"
}
