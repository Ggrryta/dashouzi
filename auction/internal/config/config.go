package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig     `mapstructure:"app"`
	Log     LogConfig     `mapstructure:"log"`
	MySQL   MySQLConfig   `mapstructure:"mysql"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Auction AuctionConfig `mapstructure:"auction"`
}

type AppConfig struct {
	Name         string        `mapstructure:"name"`
	Env          string        `mapstructure:"env"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
}

func (c MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database, c.Charset)
}

type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type AuctionConfig struct {
	BidIntervalSeconds       int           `mapstructure:"bid_interval_seconds"`
	HeartbeatIntervalSeconds int           `mapstructure:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds  int           `mapstructure:"heartbeat_timeout_seconds"`
	ScannerIntervalSeconds   int           `mapstructure:"scanner_interval_seconds"`
	SnapshotCacheMs          int           `mapstructure:"snapshot_cache_ms"`
	MaxRoomOnline            int           `mapstructure:"max_room_online"`
}

func (c AuctionConfig) BidInterval() time.Duration {
	return time.Duration(c.BidIntervalSeconds) * time.Second
}

func (c AuctionConfig) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalSeconds) * time.Second
}

func (c AuctionConfig) HeartbeatTimeout() time.Duration {
	return time.Duration(c.HeartbeatTimeoutSeconds) * time.Second
}

func (c AuctionConfig) ScannerInterval() time.Duration {
	return time.Duration(c.ScannerIntervalSeconds) * time.Second
}

func (c AuctionConfig) SnapshotCache() time.Duration {
	return time.Duration(c.SnapshotCacheMs) * time.Millisecond
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &cfg, nil
}
