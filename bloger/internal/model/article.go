package model

import "time"

type Article struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"not null;index" json:"author_id"`
	Author      User      `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Title       string    `gorm:"size:256;not null" json:"title"`
	Slug        string    `gorm:"uniqueIndex;size:256;not null" json:"slug"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	Summary     string    `gorm:"size:512" json:"summary"`
	CoverURL    string    `gorm:"size:512" json:"cover_url"`
	Status      string    `gorm:"size:16;not null;default:draft;index" json:"status"`
	ViewCount   int64     `gorm:"not null;default:0" json:"view_count"`
	IsTop       bool      `gorm:"not null;default:false" json:"is_top"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []Tag     `gorm:"many2many:article_tags" json:"tags,omitempty"`
}
