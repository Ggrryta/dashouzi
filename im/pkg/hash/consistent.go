package hash

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

type Consistent struct {
	mu           sync.RWMutex
	hashRing     []uint32         // 哈希环，排序
	hashMap      map[uint32]string // hash → node name
	virtualNodes int
}

func New(virtualNodes int) *Consistent {
	return &Consistent{
		hashMap:      make(map[uint32]string),
		virtualNodes: virtualNodes,
	}
}

func (c *Consistent) Add(nodes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range nodes {
		for i := 0; i < c.virtualNodes; i++ {
			hash := c.hashKey(node + "-" + strconv.Itoa(i))
			c.hashRing = append(c.hashRing, hash)
			c.hashMap[hash] = node
		}
	}
	sort.Slice(c.hashRing, func(i, j int) bool {
		return c.hashRing[i] < c.hashRing[j]
	})
}

func (c *Consistent) Remove(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var newRing []uint32
	for _, h := range c.hashRing {
		if c.hashMap[h] != node {
			newRing = append(newRing, h)
		} else {
			delete(c.hashMap, h)
		}
	}
	c.hashRing = newRing
}

func (c *Consistent) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.hashRing) == 0 {
		return ""
	}

	hash := c.hashKey(key)
	idx := sort.Search(len(c.hashRing), func(i int) bool {
		return c.hashRing[i] >= hash
	})

	if idx == len(c.hashRing) {
		idx = 0
	}

	return c.hashMap[c.hashRing[idx]]
}

func (c *Consistent) hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}
