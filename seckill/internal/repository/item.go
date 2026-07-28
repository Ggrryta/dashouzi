package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"seckill/internal/model"
)

type ItemRepository interface {
	Create(ctx context.Context, item *model.SeckillItem) error
	FindByID(ctx context.Context, id uint) (*model.SeckillItem, error)
	FindBySession(ctx context.Context, sessionID uint) ([]*model.SeckillItem, error)
	List(ctx context.Context) ([]*model.SeckillItem, error)
}

// RedisClient 抽象 Redis 操作接口，方便单元测试 mock
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
}

// RedisCmdClient 扩展 Redis 接口，支持 Lua 脚本 + Set 操作（秒杀用）
type RedisCmdClient interface {
	RedisClient
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (int64, error)
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
}

type itemRepo struct {
	db *gorm.DB
}

func NewItemRepo(db *gorm.DB) ItemRepository {
	return &itemRepo{db: db}
}

func (r *itemRepo) Create(_ context.Context, item *model.SeckillItem) error {
	return r.db.Create(item).Error
}

func (r *itemRepo) FindByID(_ context.Context, id uint) (*model.SeckillItem, error) {
	var item model.SeckillItem
	err := r.db.First(&item, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *itemRepo) FindBySession(_ context.Context, sessionID uint) ([]*model.SeckillItem, error) {
	var items []*model.SeckillItem
	err := r.db.Where("session_id = ?", sessionID).Find(&items).Error
	return items, err
}

func (r *itemRepo) List(_ context.Context) ([]*model.SeckillItem, error) {
	var items []*model.SeckillItem
	err := r.db.Order("created_at DESC").Find(&items).Error
	return items, err
}
