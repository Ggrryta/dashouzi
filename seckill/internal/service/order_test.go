package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"seckill/internal/model"
)

type mockOrderRepo struct {
	mu     sync.Mutex
	orders map[string]*model.SeckillOrder
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[string]*model.SeckillOrder)}
}

func (m *mockOrderRepo) Create(_ context.Context, order *model.SeckillOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(rune(order.UserID)) + "-" + string(rune(order.ItemID))
	m.orders[key] = order
	return nil
}

func (m *mockOrderRepo) FindByUserItem(_ context.Context, userID, itemID uint) (*model.SeckillOrder, error) {
	return nil, nil
}

func TestOrderCreate_Success(t *testing.T) {
	repo := newMockOrderRepo()
	svc := NewOrderService(repo)

	err := svc.Create(context.Background(), &model.SeckillOrder{
		UserID: 1, ItemID: 1, SessionID: 1, Price: 1.00,
	})
	assert.NoError(t, err)
}

func TestOrderCreate_Duplicate(t *testing.T) {
	repo := newMockOrderRepo()
	svc := NewOrderService(repo)

	svc.Create(context.Background(), &model.SeckillOrder{
		UserID: 1, ItemID: 1, SessionID: 1, Price: 1.00,
	})
	// 重复应被忽略（不 panic）
	err := svc.Create(context.Background(), &model.SeckillOrder{
		UserID: 1, ItemID: 1, SessionID: 1, Price: 1.00,
	})
	assert.NoError(t, err)
}
