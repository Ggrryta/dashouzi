package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"seckill/internal/model"
	"seckill/internal/repository"
)

type CreateSessionInput struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
}

type SessionService struct {
	repo repository.SessionRepository
}

func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) Create(ctx context.Context, input CreateSessionInput) (*model.SeckillSession, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, errors.New("name is required")
	}
	if !input.EndTime.After(input.StartTime) {
		return nil, errors.New("end time must be after start time")
	}

	session := &model.SeckillSession{
		Name:      input.Name,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
		Status:    "pending",
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SessionService) GetByID(ctx context.Context, id uint) (*model.SeckillSession, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *SessionService) FindActive(ctx context.Context) ([]*model.SeckillSession, error) {
	return s.repo.FindActive(ctx)
}

func (s *SessionService) List(ctx context.Context) ([]*model.SeckillSession, error) {
	return s.repo.List(ctx)
}
