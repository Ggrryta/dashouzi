package timeline

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// InboxProvider 收件箱接口
type InboxProvider interface {
	GetRecent(ctx context.Context, userID int64, limit int64, cursor float64) ([]int64, error)
	Count(ctx context.Context, userID int64) (int64, error)
}

// OutboxProvider 发件箱接口
type OutboxProvider interface {
	GetRange(ctx context.Context, userID int64, start, stop int64) ([]int64, error)
	Count(ctx context.Context, userID int64) (int64, error)
}

// PostFiller 帖子详情填充器
type PostFiller interface {
	GetByID(ctx context.Context, id int64) (*postInfo, error)
}

// SocialProvider 社交关系接口
type SocialProvider interface {
	GetFollowing(ctx context.Context, userID int64) ([]int64, error)
	IsBigV(ctx context.Context, userID int64) bool
}

// postInfo 帖子简要信息
type postInfo struct {
	ID        int64
	UserID    int64
	Content   string
	CreatedAt time.Time
}

// Service 时间线服务
type Service struct {
	inbox  InboxProvider
	outbox OutboxProvider
	filler PostFiller
	social SocialProvider
}

// NewService 创建时间线服务
func NewService(inbox InboxProvider, outbox OutboxProvider, filler PostFiller, social SocialProvider) *Service {
	return &Service{inbox: inbox, outbox: outbox, filler: filler, social: social}
}

// TimelineResponse 时间线响应
type TimelineResponse struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// GetTimeline 拉取用户时间线（Push + Pull 混合）
func (s *Service) GetTimeline(ctx context.Context, userID int64, limit int, cursor string) (*TimelineResponse, error) {
	var cursorFloat float64
	if cursor != "" {
		fmt.Sscanf(cursor, "%f", &cursorFloat)
	}

	// Step 1: 从收件箱读取 Push 内容（多取 1 条判段是否还有更多）
	inboxIDs, err := s.inbox.GetRecent(ctx, userID, int64(limit+1), cursorFloat)
	if err != nil {
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	inboxItems := s.fillPosts(ctx, inboxIDs)

	// Step 2: 收件箱数据已足够
	if len(inboxItems) >= limit {
		resp := &TimelineResponse{Items: inboxItems[:limit]}
		if len(inboxItems) > limit {
			resp.HasMore = true
			resp.NextCursor = formatCursor(inboxItems[limit-1].Timestamp)
		}
		return resp, nil
	}

	// Step 3: 收件箱不足，Pull 大V 发件箱
	following, err := s.social.GetFollowing(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get following: %w", err)
	}

	var allItems []Item
	allItems = append(allItems, inboxItems...)

	// 逐个拉取大V 发件箱
	for _, followeeID := range following {
		if !s.social.IsBigV(ctx, followeeID) {
			continue // 普通用户已经在收件箱里了
		}
		ids, err := s.outbox.GetRange(ctx, followeeID, 0, -1)
		if err != nil {
			continue
		}
		// 过滤 cursor 之前的数据
		var filteredIDs []int64
		for _, id := range ids {
			// 需要判断 timestamp，这里简化处理：全部拉取然后 merge 时裁剪
			filteredIDs = append(filteredIDs, id)
		}
		outboxItems := s.fillPosts(ctx, filteredIDs)
		allItems = append(allItems, outboxItems...)
	}

	// Step 4: 去重 + 排序（按时间倒序）
	allItems = deduplicate(allItems)
	sortByTimestampDesc(allItems)

	// Step 5: 取 Top N
	if len(allItems) > limit {
		allItems = allItems[:limit]
	}

	resp := &TimelineResponse{Items: allItems}
	if len(allItems) == limit {
		resp.HasMore = true
		resp.NextCursor = formatCursor(allItems[len(allItems)-1].Timestamp)
	}
	return resp, nil
}

// fillPosts 批量填充帖子详情
func (s *Service) fillPosts(ctx context.Context, ids []int64) []Item {
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		info, err := s.filler.GetByID(ctx, id)
		if err != nil || info == nil {
			continue
		}
		items = append(items, Item{
			PostID:    info.ID,
			UserID:    info.UserID,
			Content:   info.Content,
			Timestamp: float64(info.CreatedAt.Unix()),
			CreatedAt: info.CreatedAt.Format(time.RFC3339),
		})
	}
	return items
}

func deduplicate(items []Item) []Item {
	seen := make(map[int64]bool)
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if !seen[item.PostID] {
			seen[item.PostID] = true
			result = append(result, item)
		}
	}
	return result
}

func sortByTimestampDesc(items []Item) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Timestamp > items[i].Timestamp {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func formatCursor(ts float64) string {
	return strconv.FormatFloat(ts, 'f', 0, 64)
}
