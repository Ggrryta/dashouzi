package model

import "time"

type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_article;not null" json:"user_id"`
	ArticleID uint      `gorm:"uniqueIndex:idx_user_article;not null" json:"article_id"`
	Article   Article   `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
