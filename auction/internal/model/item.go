package model

import "time"

// ItemStatus 商品状态
type ItemStatus string

const (
	ItemStatusPending ItemStatus = "pending"
	ItemStatusLive    ItemStatus = "live"
	ItemStatusClosing ItemStatus = "closing"
	ItemStatusSold    ItemStatus = "sold"
	ItemStatusFailed  ItemStatus = "failed"
)

// Item 商品模型
type Item struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RoomID       int64      `gorm:"column:room_id;not null" json:"roomId"`
	SellerID     int64      `gorm:"column:seller_id;not null" json:"sellerId"`
	Title        string     `gorm:"column:title;size:128;not null" json:"title"`
	Description  string     `gorm:"column:description;size:2048" json:"description"`
	ImageURL     string     `gorm:"column:image_url;size:512" json:"imageUrl"`
	StartPrice   int64      `gorm:"column:start_price;not null" json:"startPrice"`
	MinIncrement int64      `gorm:"column:min_increment;not null" json:"minIncrement"`
	Status       ItemStatus `gorm:"column:status;size:16;not null;default:pending" json:"status"`
	StartTime    time.Time  `gorm:"column:start_time;not null" json:"startTime"`
	EndTime      time.Time  `gorm:"column:end_time;not null" json:"endTime"`
	CurrentPrice int64      `gorm:"column:current_price;not null;default:0" json:"currentPrice"`
	BidCount     int64      `gorm:"column:bid_count;not null;default:0" json:"bidCount"`
	WinnerID     *int64     `gorm:"column:winner_id" json:"winnerId"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updatedAt"`
}

func (Item) TableName() string {
	return "items"
}

// IsTerminal 是否已落槌终态
func (s ItemStatus) IsTerminal() bool {
	return s == ItemStatusSold || s == ItemStatusFailed
}

// IsBiddable 是否可出价
func (s ItemStatus) IsBiddable() bool {
	return s == ItemStatusLive || s == ItemStatusClosing
}
