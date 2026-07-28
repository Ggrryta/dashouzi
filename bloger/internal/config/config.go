package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	DB        DBConfig        `mapstructure:"db"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Log       LogConfig       `mapstructure:"log"`
	Sensitive SensitiveConfig `mapstructure:"sensitive"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	Timezone string `mapstructure:"timezone"`
}

// RedisConfig 描述 Redis 连接参数。
//
// DB 字段是 Redis 的逻辑数据库编号（0~15，默认 0），即同一实例内的 keyspace 隔离下标。
// ⚠ 现代实践已不再推荐使用多 DB 模式，原因：
//   1. 没有真正的资源隔离——16 个 DB 共享同一份内存、CPU、持久化文件、maxmemory 与 LRU。
//   2. 一个 DB 的热 key / OOM / 慢命令会拖累所有 DB。
//   3. 数量硬上限 16，无法支撑多租户或大规模业务隔离。
//   4. Redis Cluster 模式强制只允许使用 DB 0，执行 SELECT N (N>0) 直接报错，
//      未来从单机迁移到 Cluster 时该字段会被忽略，必须改造为 key 前缀隔离。
//
// 推荐做法：
//   - 生产环境改用 key 前缀隔离业务命名空间（如 "user:1"、"session:abc"），
//     需要资源隔离时拆分为多个独立 Redis 实例。
//   - 本字段仅在开发环境、小规模单机部署、同业务子系统内部做隔离时使用。
//
// 默认值 0 即可满足绝大多数场景。
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type SensitiveConfig struct {
	Words []string `mapstructure:"words"`
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, d.Timezone,
	)
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 环境变量覆盖
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	bindEnvs(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	// 环境变量可覆盖 yaml 配置
	v.BindEnv("server.host", "SERVER_HOST")
	v.BindEnv("server.port", "SERVER_PORT")
	v.BindEnv("server.mode", "SERVER_MODE", "GIN_MODE")
	v.BindEnv("db.host", "DB_HOST")
	v.BindEnv("db.port", "DB_PORT")
	v.BindEnv("db.user", "DB_USER")
	v.BindEnv("db.password", "DB_PASSWORD")
	v.BindEnv("db.dbname", "DB_NAME")
	v.BindEnv("db.sslmode", "DB_SSLMODE")
	v.BindEnv("db.timezone", "DB_TIMEZONE")
	v.BindEnv("redis.host", "REDIS_HOST")
	v.BindEnv("redis.port", "REDIS_PORT")
	v.BindEnv("redis.password", "REDIS_PASSWORD")
	v.BindEnv("redis.db", "REDIS_DB")
	v.BindEnv("jwt.secret", "JWT_SECRET")
	v.BindEnv("jwt.expire_hours", "JWT_EXPIRE_HOURS")
	v.BindEnv("log.level", "LOG_LEVEL")
	v.BindEnv("log.format", "LOG_FORMAT")
}
