package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/click33/sa-token-go/core/adapter"
)

// Constants for session keys | Session键常量
const (
	SessionKeyPrefix = "session:" // Storage key prefix | 存储键前缀
)

// Error variables | 错误变量
var (
	ErrSessionNotFound    = fmt.Errorf("session not found")
	ErrInvalidSessionData = fmt.Errorf("invalid session data")
)

// Session Session object for storing user data | 会话对象，用于存储用户数据
type Session struct {
	ID         string          `json:"id"`         // Session ID | Session标识
	CreateTime int64           `json:"createTime"` // Creation time | 创建时间
	Data       map[string]any  `json:"data"`       // Session data | 数据
	mu         sync.RWMutex    `json:"-"`          // Read-write lock | 读写锁
	storage    adapter.Storage `json:"-"`          // Storage backend | 存储
	prefix     string          `json:"-"`          // Key prefix | 键前缀
}

// NewSession Creates a new session | 创建新的Session
func NewSession(id string, storage adapter.Storage, prefix string) *Session {
	return &Session{
		ID:         id,
		CreateTime: time.Now().Unix(),
		Data:       make(map[string]any),
		storage:    storage,
		prefix:     prefix,
	}
}

// ============ Data Operations | 数据操作 ============

// Set Sets value | 设置值
func (s *Session) Set(key string, value any) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data[key] = value
	return s.save()
}

// Get Gets value | 获取值
func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.Data[key]
	return value, exists
}

// GetString gets string value | 获取字符串值
func (s *Session) GetString(key string) string {
	if value, exists := s.Get(key); exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt gets integer value | 获取整数值
func (s *Session) GetInt(key string) int {
	if value, exists := s.Get(key); exists {
		switch v := value.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// GetInt64 获取int64值
func (s *Session) GetInt64(key string) int64 {
	if value, exists := s.Get(key); exists {
		switch v := value.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

// GetBool 获取布尔值
func (s *Session) GetBool(key string) bool {
	if value, exists := s.Get(key); exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// Has 检查键是否存在
func (s *Session) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.Data[key]
	return exists
}

// Delete 删除键
func (s *Session) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Data, key)
	return s.save()
}

// Clear Clears all data | 清空所有数据
func (s *Session) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data = make(map[string]any)
	return s.save()
}

// Keys Gets all keys | 获取所有键
func (s *Session) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.Data))
	for key := range s.Data {
		keys = append(keys, key)
	}
	return keys
}

// Size Gets data count | 获取数据数量
func (s *Session) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Data)
}

// IsEmpty Checks if session has no data | 检查Session是否为空
func (s *Session) IsEmpty() bool {
	return s.Size() == 0
}

// ============ Internal Methods | 内部方法 ============

// save Saves session to storage | 保存到存储
func (s *Session) save() error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	key := s.getStorageKey()
	return s.storage.Set(key, string(data), 0)
}

// getStorageKey Gets storage key for this session | 获取Session的存储键
func (s *Session) getStorageKey() string {
	return s.prefix + SessionKeyPrefix + s.ID
}

// ============ Static Methods | 静态方法 ============

// Load Loads session from storage | 从存储加载
func Load(id string, storage adapter.Storage, prefix string) (*Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session id cannot be empty")
	}

	key := prefix + SessionKeyPrefix + id
	data, err := storage.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrSessionNotFound
	}

	dataStr, ok := data.(string)
	if !ok {
		return nil, ErrInvalidSessionData
	}

	var session Session
	if err := json.Unmarshal([]byte(dataStr), &session); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSessionData, err)
	}

	session.storage = storage
	session.prefix = prefix
	return &session, nil
}

// Destroy Destroys session | 销毁Session
func (s *Session) Destroy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getStorageKey()
	return s.storage.Delete(key)
}
