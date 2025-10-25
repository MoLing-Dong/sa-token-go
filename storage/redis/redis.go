package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/click33/sa-token-go/core/adapter"
	"github.com/redis/go-redis/v9"
)

// Storage Redis存储实现
type Storage struct {
	client    *redis.Client
	ctx       context.Context
	opTimeout time.Duration
}

// Config Redis配置
type Config struct {
	Host     string
	Port     int
	Password string
	Database int
	PoolSize int
	// Optional timeouts for redis client
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	// OperationTimeout applies to each single storage operation context
	OperationTimeout time.Duration
}

// NewStorage 通过Redis URL创建存储
func NewStorage(url string) (adapter.Storage, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Storage{
		client:    client,
		ctx:       ctx,
		opTimeout: 3 * time.Second,
	}, nil
}

// NewStorageFromConfig 通过配置创建存储
func NewStorageFromConfig(cfg *Config) (adapter.Storage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.Database,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	opTimeout := cfg.OperationTimeout
	if opTimeout <= 0 {
		opTimeout = 3 * time.Second
	}

	return &Storage{
		client:    client,
		ctx:       ctx,
		opTimeout: opTimeout,
	}, nil
}

// NewStorageFromClient 从已有的Redis客户端创建存储
func NewStorageFromClient(client *redis.Client) adapter.Storage {
	return &Storage{
		client:    client,
		ctx:       context.Background(),
		opTimeout: 3 * time.Second,
	}
}

// getKey 获取完整的键名（Storage 层不处理前缀，前缀由 Manager 层统一管理）
func (s *Storage) getKey(key string) string {
	return key
}

// Set 设置键值对
func (s *Storage) Set(key string, value any, expiration time.Duration) error {
	ctx, cancel := s.withTimeout()
	defer cancel()
	return s.client.Set(ctx, s.getKey(key), value, expiration).Err()
}

// Get 获取值
func (s *Storage) Get(key string) (any, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()
	val, err := s.client.Get(ctx, s.getKey(key)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Delete 删除键
func (s *Storage) Delete(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.getKey(key)
	}
	return s.client.Del(ctx, fullKeys...).Err()
}

// Exists 检查键是否存在
func (s *Storage) Exists(key string) bool {
	ctx, cancel := s.withTimeout()
	defer cancel()
	result, err := s.client.Exists(ctx, s.getKey(key)).Result()
	if err != nil {
		return false
	}
	return result > 0
}

// Keys 获取匹配模式的所有键
func (s *Storage) Keys(pattern string) ([]string, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	var (
		cursor uint64
		result []string
	)

	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			return nil, err
		}
		if len(keys) > 0 {
			result = append(result, keys...)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return result, nil
}

// Expire 设置键的过期时间
func (s *Storage) Expire(key string, expiration time.Duration) error {
	ctx, cancel := s.withTimeout()
	defer cancel()
	return s.client.Expire(ctx, s.getKey(key), expiration).Err()
}

// TTL 获取键的剩余生存时间
func (s *Storage) TTL(key string) (time.Duration, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()
	return s.client.TTL(ctx, s.getKey(key)).Result()
}

// Clear 清空所有数据（⚠️ 警告：会清空整个 Redis，谨慎使用！应由 Manager 层控制）
func (s *Storage) Clear() error {
	ctx, cancel := s.withTimeout()
	defer cancel()

	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, "*", 1000).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			// Use UNLINK for async non-blocking deletion
			if err := s.client.Unlink(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Ping 检查连接
func (s *Storage) Ping() error {
	ctx, cancel := s.withTimeout()
	defer cancel()
	return s.client.Ping(ctx).Err()
}

// Close 关闭连接
func (s *Storage) Close() error {
	return s.client.Close()
}

// GetClient 获取Redis客户端（用于高级操作）
func (s *Storage) GetClient() *redis.Client {
	return s.client
}

// withTimeout returns a context with the configured per-operation timeout.
func (s *Storage) withTimeout() (context.Context, context.CancelFunc) {
	if s.opTimeout > 0 {
		return context.WithTimeout(s.ctx, s.opTimeout)
	}
	return context.WithCancel(s.ctx)
}

// Builder Redis存储构建器
type Builder struct {
	host     string
	port     int
	password string
	database int
	poolSize int
}

// NewBuilder 创建构建器
func NewBuilder() *Builder {
	return &Builder{
		host:     "localhost",
		port:     6379,
		password: "",
		database: 0,
		poolSize: 10,
	}
}

// Host 设置主机
func (b *Builder) Host(host string) *Builder {
	b.host = host
	return b
}

// Port 设置端口
func (b *Builder) Port(port int) *Builder {
	b.port = port
	return b
}

// Password 设置密码
func (b *Builder) Password(password string) *Builder {
	b.password = password
	return b
}

// Database 设置数据库
func (b *Builder) Database(database int) *Builder {
	b.database = database
	return b
}

// PoolSize 设置连接池大小
func (b *Builder) PoolSize(poolSize int) *Builder {
	b.poolSize = poolSize
	return b
}

// Build 构建存储
func (b *Builder) Build() (adapter.Storage, error) {
	return NewStorageFromConfig(&Config{
		Host:     b.host,
		Port:     b.port,
		Password: b.password,
		Database: b.database,
		PoolSize: b.poolSize,
	})
}
