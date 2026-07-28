package repository

import (
	"context"

	"gorm.io/gorm"

	"seckill/internal/model"
)

type SessionRepository interface {
	Create(ctx context.Context, session *model.SeckillSession) error
	FindByID(ctx context.Context, id uint) (*model.SeckillSession, error)
	FindActive(ctx context.Context) ([]*model.SeckillSession, error)
	List(ctx context.Context) ([]*model.SeckillSession, error)
}

type sessionRepo struct {
	db *gorm.DB
}

func NewSessionRepo(db *gorm.DB) SessionRepository {
	return &sessionRepo{db: db}
}

func (r *sessionRepo) Create(_ context.Context, s *model.SeckillSession) error {
	return r.db.Create(s).Error
}

func (r *sessionRepo) FindByID(_ context.Context, id uint) (*model.SeckillSession, error) {
	var s model.SeckillSession
	err := r.db.First(&s, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *sessionRepo) FindActive(_ context.Context) ([]*model.SeckillSession, error) {
	var sessions []*model.SeckillSession
	err := r.db.Where("status = ? AND start_time <= NOW() AND end_time >= NOW()", "active").
		Find(&sessions).Error
	return sessions, err
}

func (r *sessionRepo) List(_ context.Context) ([]*model.SeckillSession, error) {
	var sessions []*model.SeckillSession
	err := r.db.Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}
