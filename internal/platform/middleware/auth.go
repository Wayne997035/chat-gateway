package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// JWTMiddleware JWT 驗證中間件（待整合 user 服務）
// 目前不啟用，等待 user 服務實現後再啟用
type JWTMiddleware struct {
	secretKey string
	enabled   bool
}

// NewJWTMiddleware 創建 JWT 中間件
func NewJWTMiddleware(secretKey string, enabled bool) *JWTMiddleware {
	return &JWTMiddleware{
		secretKey: secretKey,
		enabled:   enabled,
	}
}

// GinMiddleware Gin HTTP 中間件
// 使用方式：router.Use(jwtMiddleware.GinMiddleware())
func (m *JWTMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未啟用，直接放行
		if !m.enabled {
			c.Next()
			return
		}

		// 從 Header 獲取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "未提供認證 token"})
			c.Abort()
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "無效的認證格式"})
			c.Abort()
			return
		}

		token := parts[1]

		// TODO: 待 user 服務實現後，調用 user 服務驗證 token
		// userID, err := m.validateToken(token)
		// if err != nil {
		//     c.JSON(401, gin.H{"error": "認證失敗"})
		//     c.Abort()
		//     return
		// }

		// 將用戶 ID 存入 context
		// c.Set("user_id", userID)
		_ = token // 暫時避免 unused variable 警告

		c.Next()
	}
}

// GRPCUnaryInterceptor gRPC 一元 RPC 攔截器
// 使用方式：grpc.NewServer(grpc.UnaryInterceptor(jwtMiddleware.GRPCUnaryInterceptor()))
func (m *JWTMiddleware) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 如果未啟用，直接放行
		if !m.enabled {
			return handler(ctx, req)
		}

		// 從 metadata 獲取 token
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "未提供認證信息")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "未提供認證 token")
		}

		token := values[0]
		// 移除 "Bearer " 前綴
		token = strings.TrimPrefix(token, "Bearer ")

		// TODO: 待 user 服務實現後，調用 user 服務驗證 token
		// userID, err := m.validateToken(token)
		// if err != nil {
		//     return nil, status.Errorf(codes.Unauthenticated, "認證失敗")
		// }

		// 將用戶 ID 存入 context
		// ctx = context.WithValue(ctx, "user_id", userID)
		_ = token // 暫時避免 unused variable 警告

		return handler(ctx, req)
	}
}

// GRPCStreamInterceptor gRPC 流式 RPC 攔截器
func (m *JWTMiddleware) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// 如果未啟用，直接放行
		if !m.enabled {
			return handler(srv, ss)
		}

		// 從 metadata 獲取 token
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Errorf(codes.Unauthenticated, "未提供認證信息")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return status.Errorf(codes.Unauthenticated, "未提供認證 token")
		}

		token := values[0]
		token = strings.TrimPrefix(token, "Bearer ")

		// TODO: 待 user 服務實現後，調用 user 服務驗證 token
		// userID, err := m.validateToken(token)
		// if err != nil {
		//     return status.Errorf(codes.Unauthenticated, "認證失敗")
		// }

		_ = token // 暫時避免 unused variable 警告

		return handler(srv, ss)
	}
}

// validateToken 驗證 token（待實現）
// func (m *JWTMiddleware) validateToken(token string) (string, error) {
//     // TODO: 調用 user 服務的 gRPC API 驗證 token
//     // 1. 連接到 user 服務
//     // 2. 調用 ValidateToken RPC
//     // 3. 返回用戶 ID
//     return "", nil
// }

