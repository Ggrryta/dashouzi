package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
	"bloger/internal/service"
	"bloger/pkg/jwt"
	"bloger/pkg/logger"
	"bloger/pkg/response"
)

func init() {
	logger.Init("error", "json")
}

type stubUserRepo struct {
	users map[string]*model.User
}

func (s *stubUserRepo) Create(_ context.Context, user *model.User) error {
	user.ID = uint(len(s.users) + 1)
	s.users[user.Email] = user
	return nil
}

func (s *stubUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	return s.users[email], nil
}

func (s *stubUserRepo) FindByUsername(_ context.Context, username string) (*model.User, error) {
	return nil, nil
}

func (s *stubUserRepo) FindByID(_ context.Context, id uint) (*model.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func setupHandler() (*gin.Engine, *UserHandler) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	repo := &stubUserRepo{users: make(map[string]*model.User)}
	svc := service.NewUserService(repo)
	j := jwt.New("test", 24)
	return r, NewUserHandler(svc, j)
}

func TestRegister_InvalidJSON(t *testing.T) {
	r, h := setupHandler()
	r.POST("/register", h.Register)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer([]byte("bad json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_MissingFields(t *testing.T) {
	r, h := setupHandler()
	r.POST("/register", h.Register)

	body, _ := json.Marshal(map[string]string{"username": "test"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_EmptyBody(t *testing.T) {
	r, h := setupHandler()
	r.POST("/login", h.Login)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/login", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_InvalidJSON(t *testing.T) {
	r, h := setupHandler()
	r.POST("/login", h.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) {
		response.Success(c, "pong")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ping", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegister_Success(t *testing.T) {
	r, h := setupHandler()
	r.POST("/register", h.Register)

	body, _ := json.Marshal(map[string]string{
		"username": "newuser",
		"email":    "new@test.com",
		"password": "password123",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

func TestLogin_Success(t *testing.T) {
	r, h := setupHandler()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)

	// 先注册
	regBody, _ := json.Marshal(map[string]string{
		"username": "logintest", "email": "login@test.com", "password": "pass1234",
	})
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/register", bytes.NewBuffer(regBody))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)

	// 再登录
	loginBody, _ := json.Marshal(map[string]string{
		"email": "login@test.com", "password": "pass1234",
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/login", bytes.NewBuffer(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["data"].(map[string]interface{})["token"])
}

func TestLogin_WrongPassword(t *testing.T) {
	r, h := setupHandler()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)

	// 注册
	regBody, _ := json.Marshal(map[string]string{
		"username": "wrongpw", "email": "wp@test.com", "password": "correct",
	})
	req1 := httptest.NewRequest("POST", "/register", bytes.NewBuffer(regBody))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req1)

	// 错误密码登录
	loginBody, _ := json.Marshal(map[string]string{
		"email": "wp@test.com", "password": "wrong",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEqual(t, float64(0), resp["code"])
}
