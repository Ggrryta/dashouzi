package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
)

// mockRepo 模拟 UserRepository
type mockUserRepo struct {
	usersByEmail    map[string]*model.User
	usersByUsername map[string]*model.User
	usersByID       map[uint]*model.User
}

func newMockRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail:    make(map[string]*model.User),
		usersByUsername: make(map[string]*model.User),
		usersByID:       make(map[uint]*model.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) error {
	if _, exists := m.usersByEmail[user.Email]; exists {
		return ErrEmailExists
	}
	if _, exists := m.usersByUsername[user.Username]; exists {
		return ErrUsernameExists
	}
	user.ID = uint(len(m.usersByID) + 1)
	m.usersByEmail[user.Email] = user
	m.usersByUsername[user.Username] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByUsername(_ context.Context, username string) (*model.User, error) {
	u, ok := m.usersByUsername[username]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id uint) (*model.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

// ====== Register Tests ======

func TestRegister_Success(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	user, err := svc.Register(context.Background(), RegisterInput{
		Username: "alice",
		Email:    "alice@test.com",
		Password: "secure123",
	})

	assert.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "alice@test.com", user.Email)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, "secure123", user.PasswordHash) // 不是明文
	assert.Equal(t, "reader", user.Role)               // 默认角色
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "123456",
	})

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "bob", Email: "a@test.com", Password: "123456",
	})

	assert.ErrorIs(t, err, ErrEmailExists)
}

func TestRegister_UsernameAlreadyExists(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "123456",
	})

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "b@test.com", Password: "123456",
	})

	assert.ErrorIs(t, err, ErrUsernameExists)
}

func TestRegister_EmailRequired(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "", Password: "123456",
	})

	assert.Error(t, err)
}

func TestRegister_UsernameRequired(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "", Email: "a@test.com", Password: "123456",
	})

	assert.Error(t, err)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "123",
	})

	assert.Error(t, err)
}

// ====== Login Tests ======

func TestLogin_Success(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "secure123",
	})

	user, err := svc.Login(context.Background(), LoginInput{
		Email: "a@test.com", Password: "secure123",
	})

	assert.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.NotEmpty(t, user.PasswordHash)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "secure123",
	})

	_, err := svc.Login(context.Background(), LoginInput{
		Email: "a@test.com", Password: "wrongpass",
	})

	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.Login(context.Background(), LoginInput{
		Email: "nobody@test.com", Password: "123456",
	})

	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_EmailRequired(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.Login(context.Background(), LoginInput{
		Email: "", Password: "123456",
	})

	assert.Error(t, err)
}

// ====== GetByID Tests ======

func TestGetByID_Success(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	created, _ := svc.Register(context.Background(), RegisterInput{
		Username: "alice", Email: "a@test.com", Password: "123456",
	})

	found, err := svc.GetByID(context.Background(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "alice", found.Username)
}

func TestGetByID_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewUserService(repo)

	_, err := svc.GetByID(context.Background(), 999)
	assert.Error(t, err)
}
