package model

import "time"

type Like struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:idx_user_target;not null" json:"user_id"`
	TargetType string    `gorm:"uniqueIndex:idx_user_target;size:16;not null" json:"target_type"`
	TargetID   uint      `gorm:"uniqueIndex:idx_user_target;not null" json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}
