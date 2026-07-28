package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"seckill/internal/model"
)

type mockSessionRepo struct {
	sessions map[uint]*model.SeckillSession
	nextID   uint
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: make(map[uint]*model.SeckillSession), nextID: 1}
}

func (m *mockSessionRepo) Create(_ context.Context, s *model.SeckillSession) error {
	s.ID = m.nextID
	m.nextID++
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionRepo) FindByID(_ context.Context, id uint) (*model.SeckillSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockSessionRepo) FindActive(_ context.Context) ([]*model.SeckillSession, error) {
	var result []*model.SeckillSession
	now := time.Now()
	for _, s := range m.sessions {
		if s.Status == "active" && now.After(s.StartTime) && now.Before(s.EndTime) {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) List(_ context.Context) ([]*model.SeckillSession, error) {
	var result []*model.SeckillSession
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result, nil
}

func TestSessionCreate_Success(t *testing.T) {
	svc := NewSessionService(newMockSessionRepo())
	start := time.Now().Add(time.Hour)
	end := start.Add(2 * time.Hour)

	s, err := svc.Create(context.Background(), CreateSessionInput{
		Name: "双11秒杀", StartTime: start, EndTime: end,
	})
	assert.NoError(t, err)
	assert.Equal(t, "双11秒杀", s.Name)
	assert.Equal(t, "pending", s.Status)
}

func TestSessionCreate_EndBeforeStart(t *testing.T) {
	svc := NewSessionService(newMockSessionRepo())
	start := time.Now().Add(2 * time.Hour)
	end := time.Now().Add(time.Hour)

	_, err := svc.Create(context.Background(), CreateSessionInput{
		Name: "Test", StartTime: start, EndTime: end,
	})
	assert.Error(t, err)
}

func TestSessionCreate_EmptyName(t *testing.T) {
	svc := NewSessionService(newMockSessionRepo())
	_, err := svc.Create(context.Background(), CreateSessionInput{
		Name: "", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour),
	})
	assert.Error(t, err)
}

func TestSessionFindActive(t *testing.T) {
	repo := newMockSessionRepo()
	now := time.Now()

	// 直接注入一个活跃场次到 mock
	repo.sessions[1] = &model.SeckillSession{
		ID: 1, Name: "Active", Status: "active",
		StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour),
	}
	repo.sessions[2] = &model.SeckillSession{
		ID: 2, Name: "Future", Status: "active",
		StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour),
	}

	svc := NewSessionService(repo)
	active, err := svc.FindActive(context.Background())
	assert.NoError(t, err)
	assert.Len(t, active, 1) // 只有 ID=1 的场次在活跃时间窗口内
}
