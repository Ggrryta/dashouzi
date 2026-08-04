package model

import "time"

// RoomStatus 房间状态
type RoomStatus string

const (
	RoomStatusOnline RoomStatus = "online"
	RoomStatusClosed RoomStatus = "closed"
)

// Room 房间模型
type Room struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string     `gorm:"column:name;size:128;not null" json:"name"`
	Description string     `gorm:"column:description;size:2048" json:"description"`
	OwnerID     int64      `gorm:"column:owner_id;not null" json:"ownerId"`
	Status      RoomStatus `gorm:"column:status;size:16;not null;default:online" json:"status"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updatedAt"`
}

func (Room) TableName() string {
	return "rooms"
}
