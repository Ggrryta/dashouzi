package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	_, err = f.WriteString(content)
	assert.NoError(t, err)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
server:
  host: "0.0.0.0"
  port: 8080
db:
  host: "postgres"
  port: 5432
  user: "test"
  password: "test"
  dbname: "testdb"
  sslmode: "disable"
redis:
  host: "redis"
  port: 6379
  password: ""
  db: 0
jwt:
  secret: "test-secret"
  expire_hours: 24
log:
  level: "debug"
  format: "json"
`
	path := writeTempConfig(t, yaml)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.DB.Host)
	assert.Equal(t, "test-secret", cfg.JWT.Secret)
	assert.Equal(t, 24, cfg.JWT.ExpireHours)
}

func TestLoad_FileNotExists(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestDSN_BuildsCorrectly(t *testing.T) {
	db := DBConfig{
		Host:     "10.0.0.1",
		Port:     5432,
		User:     "app",
		Password: "pass123",
		DBName:   "bloger",
		SSLMode:  "disable",
		Timezone: "Asia/Shanghai",
	}
	dsn := db.DSN()
	assert.Contains(t, dsn, "host=10.0.0.1")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=app")
	assert.Contains(t, dsn, "password=pass123")
	assert.Contains(t, dsn, "dbname=bloger")
	assert.Contains(t, dsn, "sslmode=disable")
	assert.Contains(t, dsn, "TimeZone=Asia/Shanghai")
}

func TestRedisAddr(t *testing.T) {
	r := RedisConfig{Host: "10.0.0.2", Port: 6379}
	assert.Equal(t, "10.0.0.2:6379", r.Addr())
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("DB_HOST", "10.0.0.99")
	defer os.Unsetenv("DB_HOST")

	yaml := `
server:
  host: "0.0.0.0"
  port: 8080
db:
  host: "postgres"
  port: 5432
  user: "test"
  password: "test"
  dbname: "testdb"
  sslmode: "disable"
redis:
  host: "redis"
  port: 6379
log:
  level: "info"
  format: "json"
jwt:
  secret: "test"
  expire_hours: 1
`
	path := writeTempConfig(t, yaml)
	cfg, err := Load(path)
	assert.NoError(t, err)

	// 环境变量应覆盖 yaml 中的值
	assert.Equal(t, "10.0.0.99", cfg.DB.Host)
}
