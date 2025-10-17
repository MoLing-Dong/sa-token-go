package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/click33/sa-token-go/core"
	"github.com/click33/sa-token-go/storage/redis"
	"github.com/click33/sa-token-go/stputil"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("=== Sa-Token-Go Redis Storage Example ===\n")

	// Get Redis configuration from environment variables | 从环境变量获取 Redis 配置
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")

	// Create Redis client | 创建 Redis 客户端
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
		PoolSize: 10,
	})

	// Test Redis connection | 测试 Redis 连接
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v\n", err)
	}
	fmt.Printf("✅ Connected to Redis: %s\n\n", redisAddr)

	// Initialize Sa-Token with Redis storage | 使用 Redis 存储初始化 Sa-Token
	redisStorage, err := redis.NewStorage(redisAddr, redisPassword)
	if err != nil {
		log.Fatalf("❌ Failed to create Redis storage: %v\n", err)
	}

	stputil.SetManager(
		core.NewBuilder().
			Storage(redisStorage).
			TokenName("Authorization").
			TokenStyle(core.TokenStyleRandom64).
			Timeout(3600). // 1 hour | 1小时
			IsPrintBanner(true).
			Build(),
	)

	// Test authentication | 测试认证功能
	fmt.Println("1. Login user | 登录用户")
	token, err := stputil.Login(1000)
	if err != nil {
		log.Fatalf("Login failed: %v\n", err)
	}
	fmt.Printf("✅ Login successful! Token: %s\n\n", token)

	// Check login status | 检查登录状态
	fmt.Println("2. Check login status | 检查登录状态")
	if stputil.IsLogin(token) {
		fmt.Println("✅ User is logged in\n")
	}

	// Set permissions and roles | 设置权限和角色
	fmt.Println("3. Set permissions and roles | 设置权限和角色")
	stputil.SetPermissions(1000, []string{"user:read", "user:write", "admin:*"})
	stputil.SetRoles(1000, []string{"admin", "user"})
	fmt.Println("✅ Permissions and roles set\n")

	// Check permission | 检查权限
	fmt.Println("4. Check permissions | 检查权限")
	if stputil.HasPermission(1000, "user:read") {
		fmt.Println("✅ Has permission: user:read")
	}
	if stputil.HasPermission(1000, "admin:delete") {
		fmt.Println("✅ Has permission: admin:delete (wildcard match)")
	}
	fmt.Println()

	// Check role | 检查角色
	fmt.Println("5. Check roles | 检查角色")
	if stputil.HasRole(1000, "admin") {
		fmt.Println("✅ Has role: admin")
	}
	fmt.Println()

	// Get session | 获取 Session
	fmt.Println("6. Session management | Session 管理")
	sess, _ := stputil.GetSession(1000)
	sess.Set("username", "admin")
	sess.Set("email", "admin@example.com")
	fmt.Println("✅ Session data saved")

	username := sess.GetString("username")
	fmt.Printf("   Username: %s\n\n", username)

	// Logout | 登出
	fmt.Println("7. Logout | 登出")
	stputil.Logout(1000)
	fmt.Println("✅ User logged out")

	if !stputil.IsLogin(token) {
		fmt.Println("✅ Token is now invalid\n")
	}

	// Close Redis connection | 关闭 Redis 连接
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing Redis: %v\n", err)
		}
	}()

	fmt.Println("=== Redis Example Completed ===")
	fmt.Println("\n💡 Tips:")
	fmt.Println("   • Data is persisted in Redis")
	fmt.Println("   • Survives application restarts")
	fmt.Println("   • Suitable for production environments")
	fmt.Println("   • Supports distributed deployments")
}
