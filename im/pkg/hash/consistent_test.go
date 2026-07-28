package hash

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsistent_SameUserSameNode(t *testing.T) {
	ring := New(150)
	ring.Add("node-1", "node-2", "node-3")

	n1 := ring.Get("user-42")
	n2 := ring.Get("user-42")
	assert.Equal(t, n1, n2, "same user always maps to same node")
}

func TestConsistent_EvenDistribution(t *testing.T) {
	ring := New(150)
	ring.Add("node-1", "node-2", "node-3")

	counts := map[string]int{}
	for i := 0; i < 10000; i++ {
		node := ring.Get(fmt.Sprintf("user-%d", i))
		counts[node]++
	}

	// 每个节点应在 33% ± 15%
	for _, count := range counts {
		ratio := float64(count) / 10000
		assert.Greater(t, ratio, 0.18)
		assert.Less(t, ratio, 0.48)
	}
}

func TestConsistent_RemoveNode_MinimalMigration(t *testing.T) {
	ring := New(150)
	ring.Add("node-1", "node-2", "node-3")

	// 记录分配
	before := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("u-%d", i)
		before[key] = ring.Get(key)
	}

	// 删除 node-2
	ring.Remove("node-2")

	// 统计迁移
	migrated := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("u-%d", i)
		if before[key] != ring.Get(key) {
			migrated++
		}
	}

	// 迁移率 < 70%（普通 hash 会 100%）
	assert.Less(t, migrated, 700, "migration should be minimal")
}

func TestConsistent_AddNode_MinimalMigration(t *testing.T) {
	ring := New(150)
	ring.Add("node-1", "node-2")

	before := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("u-%d", i)
		before[key] = ring.Get(key)
	}

	ring.Add("node-3")

	migrated := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("u-%d", i)
		if before[key] != ring.Get(key) {
			migrated++
		}
	}
	assert.Less(t, migrated, 700)
}
