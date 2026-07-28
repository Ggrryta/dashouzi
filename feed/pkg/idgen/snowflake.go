package idgen

import (
	"sync"
	"time"
)

const (
	epoch        = int64(1700000000000) // 2023-11-14 的毫秒时间戳
	nodeBits     = 10
	sequenceBits = 12
	maxNode      = -1 ^ (-1 << nodeBits)
	maxSequence  = -1 ^ (-1 << sequenceBits)
	timeShift    = nodeBits + sequenceBits
	nodeShift    = sequenceBits
)

// Snowflake 雪花 ID 生成器
type Snowflake struct {
	mu        sync.Mutex
	nodeID    int64
	sequence  int64
	lastStamp int64
}

// NewSnowflake 创建生成器，nodeID 范围 [0, 1023]
func NewSnowflake(nodeID int64) *Snowflake {
	if nodeID < 0 || nodeID > maxNode {
		nodeID = nodeID & maxNode
	}
	return &Snowflake{nodeID: nodeID}
}

// NextID 生成下一个唯一 ID
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if now == s.lastStamp {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// 序列号用完，等待下一毫秒
			for now <= s.lastStamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastStamp = now

	return ((now - epoch) << timeShift) |
		(s.nodeID << nodeShift) |
		s.sequence
}
