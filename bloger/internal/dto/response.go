package dto

import "time"

type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	AvatarURL string    `json:"avatar_url"`
	Bio       string    `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RegisterResponse struct {
	User UserResponse `json:"user"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type TagResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type ArticleResponse struct {
	ID          uint         `json:"id"`
	AuthorID    uint         `json:"author_id"`
	Author      UserResponse `json:"author,omitempty"`
	Title       string       `json:"title"`
	Slug        string       `json:"slug"`
	Content     string       `json:"content"`
	Summary     string       `json:"summary"`
	CoverURL    string       `json:"cover_url"`
	Status      string       `json:"status"`
	ViewCount   int64        `json:"view_count"`
	IsTop       bool         `json:"is_top"`
	PublishedAt time.Time    `json:"published_at"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Tags        []TagResponse `json:"tags,omitempty"`
}

type CommentResponse struct {
	ID        uint              `json:"id"`
	ArticleID uint              `json:"article_id"`
	ParentID  uint              `json:"parent_id,omitempty"`
	User      UserResponse      `json:"user,omitempty"`
	Content   string            `json:"content"`
	CreatedAt time.Time         `json:"created_at"`
	Replies   []CommentResponse `json:"replies,omitempty"`
}
