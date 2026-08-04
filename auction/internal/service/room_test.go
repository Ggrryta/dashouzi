package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"auction/internal/model"
	"auction/pkg/errcode"
)

type mockRoomRepo struct {
	rooms map[int64]*model.Room
	nextID int64
}

func newMockRoomRepo() *mockRoomRepo {
	return &mockRoomRepo{
		rooms: make(map[int64]*model.Room),
		nextID: 1,
	}
}

func (m *mockRoomRepo) Create(ctx context.Context, room *model.Room) error {
	room.ID = m.nextID
	m.nextID++
	m.rooms[room.ID] = room
	return nil
}

func (m *mockRoomRepo) GetByID(ctx context.Context, id int64) (*model.Room, error) {
	return m.rooms[id], nil
}

func (m *mockRoomRepo) List(ctx context.Context, cursor int64, size int) ([]*model.Room, error) {
	var list []*model.Room
	for _, room := range m.rooms {
		list = append(list, room)
	}
	return list, nil
}

func (m *mockRoomRepo) Close(ctx context.Context, id int64) error {
	if room, ok := m.rooms[id]; ok {
		room.Status = model.RoomStatusClosed
		return nil
	}
	return nil
}

func TestRoomService_Create(t *testing.T) {
	svc := NewRoomService(newMockRoomRepo())
	room, err := svc.Create(context.Background(), 1, "测试房间", "描述")
	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, "测试房间", room.Name)
	assert.Equal(t, int64(1), room.OwnerID)
	assert.Equal(t, model.RoomStatusOnline, room.Status)
}

func TestRoomService_Create_EmptyName(t *testing.T) {
	svc := NewRoomService(newMockRoomRepo())
	room, err := svc.Create(context.Background(), 1, "", "描述")
	assert.Error(t, err)
	assert.Nil(t, room)
	assert.Equal(t, errcode.ErrBadRequest.Code, err.(*errcode.ErrCode).Code)
}

func TestRoomService_Close_NotOwner(t *testing.T) {
	repo := newMockRoomRepo()
	svc := NewRoomService(repo)
	room, _ := svc.Create(context.Background(), 1, "测试房间", "")
	err := svc.Close(context.Background(), room.ID, 2)
	assert.Error(t, err)
	assert.Equal(t, errcode.ErrForbidden.Code, err.(*errcode.ErrCode).Code)
}
