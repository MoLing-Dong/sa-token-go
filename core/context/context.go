package context

import (
	"strings"

	"github.com/click33/sa-token-go/core/adapter"
	"github.com/click33/sa-token-go/core/manager"
)

const (
	bearerPrefix = "Bearer "
	authHeader   = "Authorization"
)

// SaTokenContext Sa-Token context for current request | Sa-Token上下文，用于当前请求
type SaTokenContext struct {
	ctx     adapter.RequestContext
	manager *manager.Manager
}

// NewContext creates a new Sa-Token context | 创建新的Sa-Token上下文
func NewContext(ctx adapter.RequestContext, mgr *manager.Manager) *SaTokenContext {
	return &SaTokenContext{
		ctx:     ctx,
		manager: mgr,
	}
}

// extractBearerToken 从 Authorization 头中提取 Bearer Token
func extractBearerToken(auth string) string {
	auth = strings.TrimSpace(auth)
	if auth == "" {
		return ""
	}

	// 支持大小写不敏感的 Bearer 前缀
	if len(auth) > 7 && strings.EqualFold(auth[:7], bearerPrefix) {
		return strings.TrimSpace(auth[7:])
	}

	return auth
}

// GetTokenValue gets token value from current request | 获取当前请求的Token值
func (c *SaTokenContext) GetTokenValue() string {
	cfg := c.manager.GetConfig()

	// 1. 尝试从Header获取
	if cfg.IsReadHeader {
		// 从自定义 token 名称的 Header 获取
		if token := strings.TrimSpace(c.ctx.GetHeader(cfg.TokenName)); token != "" {
			return token
		}

		// 从 Authorization 头获取
		if auth := c.ctx.GetHeader(authHeader); auth != "" {
			if token := extractBearerToken(auth); token != "" {
				return token
			}
		}
	}

	// 2. 尝试从Cookie获取
	if cfg.IsReadCookie {
		if token := strings.TrimSpace(c.ctx.GetCookie(cfg.TokenName)); token != "" {
			return token
		}
	}

	// 3. 尝试从Query参数获取
	if token := strings.TrimSpace(c.ctx.GetQuery(cfg.TokenName)); token != "" {
		return token
	}

	return ""
}

// IsLogin 检查当前请求是否已登录
func (c *SaTokenContext) IsLogin() bool {
	token := c.GetTokenValue()
	return c.manager.IsLogin(token)
}

// CheckLogin 检查登录（未登录抛出错误）
func (c *SaTokenContext) CheckLogin() error {
	token := c.GetTokenValue()
	return c.manager.CheckLogin(token)
}

// GetLoginID 获取当前登录ID
func (c *SaTokenContext) GetLoginID() (string, error) {
	token := c.GetTokenValue()
	return c.manager.GetLoginID(token)
}

// HasPermission 检查是否有指定权限
func (c *SaTokenContext) HasPermission(permission string) bool {
	loginID, err := c.GetLoginID()
	if err != nil {
		return false
	}
	return c.manager.HasPermission(loginID, permission)
}

// HasRole 检查是否有指定角色
func (c *SaTokenContext) HasRole(role string) bool {
	loginID, err := c.GetLoginID()
	if err != nil {
		return false
	}
	return c.manager.HasRole(loginID, role)
}

// GetRequestContext 获取原始请求上下文
func (c *SaTokenContext) GetRequestContext() adapter.RequestContext {
	return c.ctx
}

// GetManager 获取管理器
func (c *SaTokenContext) GetManager() *manager.Manager {
	return c.manager
}
