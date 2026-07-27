package model

import "time"

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ArticleID uint      `gorm:"not null;index" json:"article_id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ParentID  *uint     `gorm:"index" json:"parent_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	IsDeleted bool      `gorm:"not null;default:false" json:"is_deleted"`
	CreatedAt time.Time `json:"created_at"`
	Replies   []Comment `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
}
