package timeline

// Item 时间线条目
type Item struct {
	PostID    int64   `json:"post_id"`
	UserID    int64   `json:"user_id"`
	Content   string  `json:"content"`
	Timestamp float64 `json:"timestamp"`
	CreatedAt string  `json:"created_at"`
}

// Merge 归并两个已按时间倒序排列的列表
func Merge(a, b []Item) []Item {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	result := make([]Item, 0, len(a)+len(b))
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if a[i].Timestamp >= b[j].Timestamp {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}

	// 追加剩余元素
	for ; i < len(a); i++ {
		result = append(result, a[i])
	}
	for ; j < len(b); j++ {
		result = append(result, b[j])
	}

	return result
}

// MergeMulti 归并多条时间线（K 路），按时间倒序
func MergeMulti(lists [][]Item) []Item {
	if len(lists) == 0 {
		return nil
	}
	result := lists[0]
	for i := 1; i < len(lists); i++ {
		result = Merge(result, lists[i])
	}
	return result
}
