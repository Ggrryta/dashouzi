package model

import "time"

// Bid 出价记录模型
type Bid struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ItemID    int64     `gorm:"column:item_id;not null" json:"itemId"`
	BidderID  int64     `gorm:"column:bidder_id;not null" json:"bidderId"`
	Amount    int64     `gorm:"column:amount;not null" json:"amount"`
	BidTime   time.Time `gorm:"column:bid_time;not null;default:CURRENT_TIMESTAMP(3)" json:"bidTime"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (Bid) TableName() string {
	return "bids"
}
