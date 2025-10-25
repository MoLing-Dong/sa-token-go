package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/click33/sa-token-go/core/adapter"
	"github.com/click33/sa-token-go/core/config"
	"github.com/click33/sa-token-go/core/token"
)

// Refresh Token Implementation
// 刷新令牌实现
//
// Flow | 流程:
// 1. GenerateTokenPair() - Create access token + refresh token | 创建访问令牌 + 刷新令牌
// 2. Access token expires (short-lived, e.g. 2h) | 访问令牌过期（短期，如2小时）
// 3. RefreshAccessToken() - Use refresh token to get new access token | 使用刷新令牌获取新访问令牌
// 4. Refresh token expires (long-lived, 30 days) | 刷新令牌过期（长期，30天）
//
// Usage | 用法:
//   tokenInfo, _ := manager.LoginWithRefreshToken(loginID, "web")
//   // ... access token expires ...
//   newInfo, _ := manager.RefreshAccessToken(tokenInfo.RefreshToken)

// Constants for refresh token | 刷新令牌常量
const (
	DefaultRefreshTTL  = 30 * 24 * time.Hour // 30 days | 30天
	DefaultAccessTTL   = 2 * time.Hour       // 2 hours | 2小时
	RefreshTokenLength = 32                  // Refresh token byte length | 刷新令牌字节长度
	RefreshKeySuffix   = "refresh:"          // Key suffix after prefix | 前缀后的键后缀
)

// Error variables | 错误变量
var (
	ErrInvalidRefreshToken = fmt.Errorf("invalid refresh token")
	ErrRefreshTokenExpired = fmt.Errorf("refresh token expired")
	ErrInvalidRefreshData  = fmt.Errorf("invalid refresh token data")
)

// RefreshTokenInfo refresh token information | 刷新令牌信息
type RefreshTokenInfo struct {
	RefreshToken string // Refresh token (long-lived) | 刷新令牌（长期有效）
	AccessToken  string // Access token (short-lived) | 访问令牌（短期有效）
	LoginID      string // User login ID | 用户登录ID
	Device       string // Device type | 设备类型
	CreateTime   int64  // Creation timestamp | 创建时间戳
	ExpireTime   int64  // Expiration timestamp | 过期时间戳
}

// RefreshTokenManager Refresh token manager | 刷新令牌管理器
type RefreshTokenManager struct {
	storage    adapter.Storage
	keyPrefix  string // Configurable prefix | 可配置的前缀
	tokenGen   *token.Generator
	refreshTTL time.Duration // Refresh token TTL (30 days) | 刷新令牌有效期（30天）
	accessTTL  time.Duration // Access token TTL (configurable) | 访问令牌有效期（可配置）
}

// NewRefreshTokenManager Creates a new refresh token manager | 创建新的刷新令牌管理器
// prefix: key prefix (e.g., "satoken:" or "" for Java compatibility) | 键前缀（如："satoken:" 或 "" 兼容Java）
// cfg: configuration, uses Timeout for access token TTL | 配置，使用Timeout作为访问令牌有效期
func NewRefreshTokenManager(storage adapter.Storage, prefix string, cfg *config.Config) *RefreshTokenManager {
	accessTTL := time.Duration(cfg.Timeout) * time.Second

	if accessTTL == 0 {
		accessTTL = DefaultAccessTTL
	}

	return &RefreshTokenManager{
		storage:    storage,
		keyPrefix:  prefix,
		tokenGen:   token.NewGenerator(cfg),
		refreshTTL: DefaultRefreshTTL,
		accessTTL:  accessTTL,
	}
}

// GenerateTokenPair Generates access token and refresh token pair | 生成访问令牌和刷新令牌对
func (rtm *RefreshTokenManager) GenerateTokenPair(loginID, device string) (*RefreshTokenInfo, error) {
	if loginID == "" {
		return nil, fmt.Errorf("loginID cannot be empty")
	}

	// Generate access token | 生成访问令牌
	accessToken, err := rtm.tokenGen.Generate(loginID, device)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token | 生成刷新令牌
	refreshTokenBytes := make([]byte, RefreshTokenLength)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	refreshToken := hex.EncodeToString(refreshTokenBytes)

	now := time.Now()
	info := &RefreshTokenInfo{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
		LoginID:      loginID,
		Device:       device,
		CreateTime:   now.Unix(),
		ExpireTime:   now.Add(rtm.refreshTTL).Unix(),
	}

	key := rtm.getRefreshKey(refreshToken)
	if err := rtm.storage.Set(key, info, rtm.refreshTTL); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return info, nil
}

// RefreshAccessToken Generates new access token using refresh token | 使用刷新令牌生成新的访问令牌
func (rtm *RefreshTokenManager) RefreshAccessToken(refreshToken string) (*RefreshTokenInfo, error) {
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	key := rtm.getRefreshKey(refreshToken)

	data, err := rtm.storage.Get(key)
	if err != nil || data == nil {
		return nil, ErrInvalidRefreshToken
	}

	oldInfo, ok := data.(*RefreshTokenInfo)
	if !ok {
		return nil, ErrInvalidRefreshData
	}

	// Check expiration | 检查是否过期
	if time.Now().Unix() > oldInfo.ExpireTime {
		rtm.storage.Delete(key)
		return nil, ErrRefreshTokenExpired
	}

	// Generate new access token | 生成新的访问令牌
	newAccessToken, err := rtm.tokenGen.Generate(oldInfo.LoginID, oldInfo.Device)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}

	oldInfo.AccessToken = newAccessToken

	// Update storage | 更新存储
	if err := rtm.storage.Set(key, oldInfo, rtm.refreshTTL); err != nil {
		return nil, fmt.Errorf("failed to update refresh token: %w", err)
	}

	return oldInfo, nil
}

// RevokeRefreshToken Revokes a refresh token | 撤销刷新令牌
func (rtm *RefreshTokenManager) RevokeRefreshToken(refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	key := rtm.getRefreshKey(refreshToken)
	return rtm.storage.Delete(key)
}

// GetRefreshTokenInfo Gets refresh token information | 获取刷新令牌信息
func (rtm *RefreshTokenManager) GetRefreshTokenInfo(refreshToken string) (*RefreshTokenInfo, error) {
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	key := rtm.getRefreshKey(refreshToken)

	data, err := rtm.storage.Get(key)
	if err != nil || data == nil {
		return nil, ErrInvalidRefreshToken
	}

	info, ok := data.(*RefreshTokenInfo)
	if !ok {
		return nil, ErrInvalidRefreshData
	}

	return info, nil
}

// IsValid Checks if refresh token is valid | 检查刷新令牌是否有效
func (rtm *RefreshTokenManager) IsValid(refreshToken string) bool {
	info, err := rtm.GetRefreshTokenInfo(refreshToken)
	if err != nil {
		return false
	}

	return time.Now().Unix() <= info.ExpireTime
}

// getRefreshKey Gets storage key for refresh token | 获取刷新令牌的存储键
func (rtm *RefreshTokenManager) getRefreshKey(refreshToken string) string {
	return rtm.keyPrefix + RefreshKeySuffix + refreshToken
}
