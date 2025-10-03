package middleware

import (
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 認證中間件
// 等待 user 服務實現後啟用
type AuthMiddleware struct {
	enabled bool
}

// NewAuthMiddleware 創建新的認證中間件
func NewAuthMiddleware(enabled bool) *AuthMiddleware {
	return &AuthMiddleware{
		enabled: enabled,
	}
}

// ValidateToken 驗證 token 的中間件
// TODO: 待 user 服務實現後啟用
func (m *AuthMiddleware) ValidateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		// TODO: 實作 token 驗證邏輯
		// 1. 從 header 獲取 token
		// 2. 調用 user 服務驗證 token
		// 3. 將用戶信息存入 context
		// 4. 檢查權限等

		c.Next()
	}
}

// RequireAuth 要求認證的中間件（強制）
// TODO: 待 user 服務實現後使用
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 從 header 獲取並驗證 token
		// authHeader := c.GetHeader("Authorization")
		// if authHeader == "" {
		//     c.JSON(401, gin.H{"error": "未授權訪問"})
		//     c.Abort()
		//     return
		// }
		
		// TODO: 調用 user 服務驗證
		
		c.Next()
	}
}

// CheckRoomMembership 檢查聊天室成員權限
// TODO: 待實現
func CheckRoomMembership() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 檢查用戶是否是聊天室成員
		// roomID := c.Param("room_id")
		// userID := c.GetString("user_id") // 從認證中間件獲取
		
		// TODO: 調用 gRPC 檢查成員關係
		
		c.Next()
	}
}

