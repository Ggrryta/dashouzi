package model

import "time"

type SeckillSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Status    string    `gorm:"size:16;not null;default:pending" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
