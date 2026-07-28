package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"seckill/internal/model"
	"seckill/internal/repository"
)

type mockItemRepo struct {
	items    map[uint]*model.SeckillItem
	nextID   uint
}

func newMockItemRepo() *mockItemRepo {
	return &mockItemRepo{items: make(map[uint]*model.SeckillItem), nextID: 1}
}

func (m *mockItemRepo) Create(_ context.Context, item *model.SeckillItem) error {
	item.ID = m.nextID
	m.nextID++
	m.items[item.ID] = item
	return nil
}

func (m *mockItemRepo) FindByID(_ context.Context, id uint) (*model.SeckillItem, error) {
	item, ok := m.items[id]
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (m *mockItemRepo) FindBySession(_ context.Context, sessionID uint) ([]*model.SeckillItem, error) {
	var result []*model.SeckillItem
	for _, item := range m.items {
		if item.SessionID == sessionID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (m *mockItemRepo) List(_ context.Context) ([]*model.SeckillItem, error) {
	var result []*model.SeckillItem
	for _, item := range m.items {
		result = append(result, item)
	}
	return result, nil
}

type mockRedisItem struct {
	data map[string]string
	sets map[string]map[string]bool
}

func newMockRedisItem() *mockRedisItem {
	return &mockRedisItem{
		data: make(map[string]string),
		sets: make(map[string]map[string]bool),
	}
}

func (m *mockRedisItem) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	m.data[key] = fmt.Sprint(value)
	return nil
}

func (m *mockRedisItem) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}

func (m *mockRedisItem) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

// 确保接口实现
var _ repository.RedisClient = (*mockRedisItem)(nil)

func TestItemCreate_Success(t *testing.T) {
	svc := NewItemService(newMockItemRepo(), newMockSessionRepo())
	_, err := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "iPhone", Price: 1.00, OriginPrice: 9999, TotalStock: 100,
	})
	assert.NoError(t, err)
}

func TestItemCreate_PriceExceedsOrigin(t *testing.T) {
	svc := NewItemService(newMockItemRepo(), newMockSessionRepo())
	_, err := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "iPhone", Price: 9999, OriginPrice: 100, TotalStock: 100,
	})
	assert.Error(t, err)
}

func TestItemCreate_EmptyTitle(t *testing.T) {
	svc := NewItemService(newMockItemRepo(), newMockSessionRepo())
	_, err := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "", Price: 1, OriginPrice: 100, TotalStock: 100,
	})
	assert.Error(t, err)
}

func TestItemCreate_ZeroStock(t *testing.T) {
	svc := NewItemService(newMockItemRepo(), newMockSessionRepo())
	_, err := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "iPhone", Price: 1, OriginPrice: 9999, TotalStock: 0,
	})
	assert.Error(t, err)
}

// ====== Preheat Tests ======

func TestPreheat_SetsStock(t *testing.T) {
	itemRepo := newMockItemRepo()
	redisClient := newMockRedisItem()
	svc := NewItemService(itemRepo, newMockSessionRepo())

	// 创建商品
	item, _ := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "Test", Price: 1, OriginPrice: 100, TotalStock: 100,
	})

	err := PreheatStock(context.Background(), redisClient, item)
	assert.NoError(t, err)
	assert.Equal(t, "100", redisClient.data["seckill:stock:1"])
}

func TestPreheat_AlreadyPreheated(t *testing.T) {
	itemRepo := newMockItemRepo()
	redisClient := newMockRedisItem()
	svc := NewItemService(itemRepo, newMockSessionRepo())

	item, _ := svc.Create(context.Background(), CreateItemInput{
		SessionID: 1, Title: "Test", Price: 1, OriginPrice: 100, TotalStock: 100,
	})

	PreheatStock(context.Background(), redisClient, item)
	PreheatStock(context.Background(), redisClient, item) // 重复预热

	assert.Equal(t, "100", redisClient.data["seckill:stock:1"]) // 不翻倍
}
