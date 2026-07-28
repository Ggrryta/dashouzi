package model

import "time"

type SeckillOrder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:uk_user_item" json:"user_id"`
	SessionID uint      `gorm:"not null" json:"session_id"`
	ItemID    uint      `gorm:"not null;uniqueIndex:uk_user_item" json:"item_id"`
	Price     float64   `gorm:"type:decimal(10,2);not null" json:"price"`
	Status    string    `gorm:"size:16;not null;default:paid" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
