package model

import "time"

type SeckillItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SessionID   uint      `gorm:"not null;index" json:"session_id"`
	Title       string    `gorm:"size:256;not null" json:"title"`
	Price       float64   `gorm:"type:decimal(10,2);not null" json:"price"`
	OriginPrice float64   `gorm:"type:decimal(10,2);not null" json:"origin_price"`
	TotalStock  int       `gorm:"not null" json:"total_stock"`
	SoldCount   int       `gorm:"not null;default:0" json:"sold_count"`
	ImageURL    string    `gorm:"size:512" json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}
