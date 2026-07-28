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
	f.WriteString(content)
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
  host: "mysql"
  port: 3306
  user: "test"
  password: "test"
  name: "testdb"
redis:
  host: "redis"
  port: 6379
  password: ""
  db: 0
kafka:
  brokers:
    - "kafka:9092"
  topic: "test.orders"
  consumer_group: "test-group"
log:
  level: "debug"
  format: "json"
`
	cfg, err := Load(writeTempConfig(t, yaml))
	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "mysql", cfg.DB.Host)
	assert.Equal(t, "redis", cfg.Redis.Host)
	assert.Equal(t, []string{"kafka:9092"}, cfg.Kafka.Brokers)
}

func TestMySQLDSN(t *testing.T) {
	db := DBConfig{Host: "mysql", Port: 3306, User: "seckill", Password: "pass123", Name: "seckill"}
	dsn := db.DSN()
	assert.Contains(t, dsn, "tcp(mysql:3306)")
	assert.Contains(t, dsn, "seckill:pass123")
	assert.Contains(t, dsn, "charset=utf8mb4")
	assert.Contains(t, dsn, "parseTime=True")
}

func TestRedisAddr(t *testing.T) {
	r := RedisConfig{Host: "redis", Port: 6379}
	assert.Equal(t, "redis:6379", r.Addr())
}

func TestKafkaBootstrapServers(t *testing.T) {
	k := KafkaConfig{Brokers: []string{"kafka:9092"}}
	assert.Equal(t, "kafka:9092", k.BootstrapServers())
}

func TestKafkaBootstrapServers_Multiple(t *testing.T) {
	k := KafkaConfig{Brokers: []string{"kafka1:9092", "kafka2:9092"}}
	assert.Equal(t, "kafka1:9092,kafka2:9092", k.BootstrapServers())
}
