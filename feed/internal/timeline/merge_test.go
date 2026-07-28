package timeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// T3.3: 2 路归并
func TestMerge_TwoWay(t *testing.T) {
	a := []Item{
		{PostID: 1, Timestamp: 100},
		{PostID: 3, Timestamp: 80},
	}
	b := []Item{
		{PostID: 2, Timestamp: 90},
		{PostID: 4, Timestamp: 70},
	}
	result := Merge(a, b)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, int64(1), result[0].PostID)
	assert.Equal(t, int64(2), result[1].PostID)
	assert.Equal(t, int64(3), result[2].PostID)
	assert.Equal(t, int64(4), result[3].PostID)
}

// T3.4: 5 路归并
func TestMerge_FiveWay(t *testing.T) {
	lists := [][]Item{
		{{PostID: 10, Timestamp: 100}},
		{{PostID: 20, Timestamp: 101}},
		{{PostID: 30, Timestamp: 99}},
		{{PostID: 40, Timestamp: 102}},
		{{PostID: 50, Timestamp: 98}},
	}
	result := MergeMulti(lists)
	assert.Equal(t, 5, len(result))

	for i := 1; i < len(result); i++ {
		assert.True(t, result[i].Timestamp <= result[i-1].Timestamp,
			"not sorted desc: idx %d ts=%f > idx %d ts=%f",
			i, result[i].Timestamp, i-1, result[i-1].Timestamp)
	}
	assert.Equal(t, int64(40), result[0].PostID)
	assert.Equal(t, int64(50), result[4].PostID)
}

// T3.5: 空输入
func TestMerge_Empty(t *testing.T) {
	r := Merge([]Item{}, []Item{})
	assert.Empty(t, r)

	r2 := MergeMulti([][]Item{})
	assert.Empty(t, r2)
}

// T3.6: 只有 1 路有数据
func TestMerge_OneSideEmpty(t *testing.T) {
	a := []Item{{PostID: 1, Timestamp: 100}}
	r := Merge(a, []Item{})
	assert.Equal(t, 1, len(r))
	assert.Equal(t, int64(1), r[0].PostID)

	lists := [][]Item{
		{},
		{{PostID: 42, Timestamp: 999}},
		{},
	}
	r2 := MergeMulti(lists)
	assert.Equal(t, 1, len(r2))
	assert.Equal(t, int64(42), r2[0].PostID)
}

// 不等长归并
func TestMerge_UnequalLength(t *testing.T) {
	a := []Item{
		{PostID: 1, Timestamp: 100},
		{PostID: 2, Timestamp: 90},
		{PostID: 3, Timestamp: 80},
	}
	b := []Item{
		{PostID: 4, Timestamp: 95},
	}
	result := Merge(a, b)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, int64(1), result[0].PostID)
	assert.Equal(t, int64(4), result[1].PostID)
	assert.Equal(t, int64(2), result[2].PostID)
	assert.Equal(t, int64(3), result[3].PostID)
}
