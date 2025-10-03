package server

import (
	"context"
	"strconv"
	"time"

	"chat-gateway/internal/grpcclient"
	"chat-gateway/internal/httputil"
	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/health"
	"chat-gateway/internal/platform/middleware"
	"chat-gateway/proto/chat"

	"github.com/gin-gonic/gin"
)

// securityHeadersMiddleware 添加安全標頭
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止點擊劫持
		c.Header("X-Frame-Options", "DENY")

		// 防止 MIME 類型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 啟用 XSS 保護
		c.Header("X-XSS-Protection", "1; mode=block")

		// 內容安全策略
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';")

		// 強制 HTTPS（生產環境）
		// c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// 推薦政策
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 權限政策
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// Router 設定路由 - 簡化版本，只保留健康檢查
func Router() *gin.Engine {
	r := gin.Default()

	// 添加安全的 CORS 中間件
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 只允許特定的來源（生產環境應該從配置文件讀取）
		allowedOrigins := map[string]bool{
			"http://localhost:3000":  true, // 開發環境前端
			"http://localhost:8080":  true, // 本地測試
			"http://127.0.0.1:5500":  true, // Live Server
			"http://127.0.0.1:8080":  true, // 本地測試 (127.0.0.1)
			"http://localhost:5500":  true, // Live Server (localhost)
			"https://yourdomain.com": true, // 生產環境（請修改為實際域名）
		}

		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400") // 預檢請求緩存 24 小時

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 添加請求 ID 中間件（最優先）
	r.Use(middleware.RequestIDMiddleware())

	// 添加安全標頭中間件
	r.Use(securityHeadersMiddleware())

	// 添加請求元數據中間件（提取 IP、User-Agent）
	r.Use(middleware.RequestMetadataMiddleware())
	
	// 從配置讀取限制參數
	cfg := config.Get()
	
	// 添加請求大小限制（防止大文件攻擊）
	maxMemory := int64(10 << 20) // 默認 10MB
	if cfg != nil && cfg.Limits.Request.MaxMultipartMemory > 0 {
		maxMemory = cfg.Limits.Request.MaxMultipartMemory
	}
	r.MaxMultipartMemory = maxMemory

	// 創建 Rate Limiter
	defaultLimit := 100
	if cfg != nil && cfg.Limits.RateLimiting.DefaultPerMinute > 0 {
		defaultLimit = cfg.Limits.RateLimiting.DefaultPerMinute
	}
	rateLimiter := middleware.NewPerEndpointRateLimiter(defaultLimit, time.Minute)
	
	// 為不同端點設置不同的速率限制
	if cfg != nil && cfg.Limits.RateLimiting.Enabled {
		if cfg.Limits.RateLimiting.MessagesPerMin > 0 {
			rateLimiter.SetLimit("/api/v1/messages", cfg.Limits.RateLimiting.MessagesPerMin, time.Minute)
		}
		if cfg.Limits.RateLimiting.RoomsPerMin > 0 {
			rateLimiter.SetLimit("/api/v1/rooms", cfg.Limits.RateLimiting.RoomsPerMin, time.Minute)
		}
		if cfg.Limits.RateLimiting.SSEPerMin > 0 {
			rateLimiter.SetLimit("/api/v1/messages/stream", cfg.Limits.RateLimiting.SSEPerMin, time.Minute)
		}
	}
	
	// 應用 Rate Limiting 中間件
	r.Use(rateLimiter.Middleware())
	
	// 創建 SSE 連接限制器
	sseMaxPerIP := 3
	sseInterval := 10
	sseMaxTotal := 1000
	if cfg != nil {
		if cfg.Limits.SSE.MaxConnectionsPerIP > 0 {
			sseMaxPerIP = cfg.Limits.SSE.MaxConnectionsPerIP
		}
		if cfg.Limits.SSE.MinConnectionInterval > 0 {
			sseInterval = cfg.Limits.SSE.MinConnectionInterval
		}
		if cfg.Limits.SSE.MaxTotalConnections > 0 {
			sseMaxTotal = cfg.Limits.SSE.MaxTotalConnections
		}
	}
	sseLimiter := middleware.NewSSEConnectionLimiter(sseMaxPerIP, time.Duration(sseInterval)*time.Second, sseMaxTotal)

	// 創建處理器
	healthHandler := health.NewHealthHandler()

	// health check
	r.GET("/health", healthHandler.HealthCheck)

	// 添加聊天室 API 路由
	r.POST("/api/v1/rooms", createRoom)
	r.GET("/api/v1/rooms", listUserRooms)
	r.POST("/api/v1/rooms/:room_id/members", addRoomMember)
	r.DELETE("/api/v1/rooms/:room_id/members/:user_id", removeRoomMember)
	r.POST("/api/v1/messages", sendMessage)
	r.GET("/api/v1/messages", getMessages)
	r.POST("/api/v1/messages/read", markAsRead)

	// SSE endpoint - 應用額外的連接限制
	r.GET("/api/v1/messages/stream", sseLimiter.Middleware(), streamMessages)

	return r
}

// 創建聊天室
func createRoom(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		OwnerID string `json:"owner_id"`
		Members []struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		} `json:"members"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "無效的請求格式"})
		return
	}

	// 驗證聊天室名稱
	if err := middleware.ValidateRoomName(req.Name); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	// 驗證 OwnerID
	if err := middleware.ValidateUserID(req.OwnerID); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	// 驗證成員數量
	cfg := config.Get()
	maxMembers := 1000 // 默認
	if cfg != nil && cfg.Limits.Room.MaxMembers > 0 {
		maxMembers = cfg.Limits.Room.MaxMembers
	}
	if len(req.Members) > maxMembers {
		c.JSON(400, gin.H{"error": "成員數量超過限制"})
		return
	}

	// 消毒聊天室名稱
	sanitizedName := middleware.SanitizeInput(req.Name)

	// 轉換為 gRPC 請求並驗證每個成員 ID
	memberIDs := make([]string, len(req.Members))
	for i, member := range req.Members {
		if err := middleware.ValidateUserID(member.UserID); err != nil {
			c.JSON(400, gin.H{"error": "成員 ID 格式錯誤"})
			return
		}
		memberIDs[i] = member.UserID
	}

	grpcReq := &chat.CreateRoomRequest{
		Name:      sanitizedName,
		Type:      req.Type,
		OwnerId:   req.OwnerID,
		MemberIds: memberIDs,
		Settings: &chat.RoomSettings{
			AllowInvite:         true,
			AllowEditMessages:   true,
			AllowDeleteMessages: true,
			MaxMembers:          int32(maxMembers),
		},
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.CreateRoom(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": resp.Success,
		"message": resp.Message,
		"data": gin.H{
			"id":         resp.Room.Id,
			"name":       resp.Room.Name,
			"type":       resp.Room.Type,
			"owner_id":   resp.Room.OwnerId,
			"created_at": resp.Room.CreatedAt,
		},
	})
}

// 列出用戶聊天室
func listUserRooms(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(400, gin.H{"error": "缺少 user_id 參數"})
		return
	}

	// 獲取分頁參數
	cfg := config.Get()
	limit := 10 // 默認
	if cfg != nil && cfg.Limits.Pagination.DefaultPageSize > 0 {
		limit = cfg.Limits.Pagination.DefaultPageSize
	}
	cursor := c.Query("cursor")

	// 可選：解析 limit 參數
	if limitStr := c.Query("limit"); limitStr != "" {
		// 可以在這裡解析 limitStr，暫時使用默認值
	}

	grpcReq := &chat.ListUserRoomsRequest{
		UserId: userID,
		Limit:  int32(limit),
		Cursor: cursor,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.ListUserRooms(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}
	messageClient := chat.NewChatRoomServiceClient(conn)

	// 轉換響應，包含最後訊息和未讀數量
	rooms := make([]map[string]interface{}, len(resp.Rooms))
	for i, room := range resp.Rooms {
		// 獲取未讀數量
		unreadResp, _ := messageClient.GetUnreadCount(context.Background(), &chat.GetUnreadCountRequest{
			UserId: userID,
			RoomId: room.Id,
		})

		unreadCount := int32(0)
		if unreadResp != nil && unreadResp.Success {
			unreadCount = unreadResp.Count
		}

		rooms[i] = map[string]interface{}{
			"id":                room.Id,
			"name":              room.Name,
			"type":              room.Type,
			"owner_id":          room.OwnerId,
			"created_at":        room.CreatedAt,
			"updated_at":        room.UpdatedAt,
			"members":           room.Members,
			"last_message":      room.LastMessage,
			"last_message_time": room.LastMessageTime,
			"unread_count":      unreadCount,
		}
	}

	c.JSON(200, gin.H{
		"success":  resp.Success,
		"message":  resp.Message,
		"data":     rooms,
		"cursor":   resp.Cursor,
		"has_more": resp.HasMore,
	})
}

// 發送消息
func sendMessage(c *gin.Context) {
	var req struct {
		RoomID   string `json:"room_id"`
		SenderID string `json:"sender_id"`
		Content  string `json:"content"`
		Type     string `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "無效的請求格式"})
		return
	}

	// 驗證和消毒輸入
	if err := middleware.ValidateRoomID(req.RoomID); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	if err := middleware.ValidateUserID(req.SenderID); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	if err := middleware.ValidateMessageContent(req.Content); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	// 消毒輸入內容
	sanitizedContent := middleware.SanitizeInput(req.Content)

	grpcReq := &chat.SendMessageRequest{
		RoomId:   req.RoomID,
		SenderId: req.SenderID,
		Content:  sanitizedContent,
		Type:     req.Type,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.SendMessage(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": resp.Success,
		"message": resp.Message,
		"data": gin.H{
			"id":         resp.ChatMessage.Id,
			"room_id":    resp.ChatMessage.RoomId,
			"sender_id":  resp.ChatMessage.SenderId,
			"content":    resp.ChatMessage.Content,
			"type":       resp.ChatMessage.Type,
			"created_at": resp.ChatMessage.CreatedAt,
		},
	})
}

// 獲取消息
func getMessages(c *gin.Context) {
	roomID := c.Query("room_id")
	userID := c.Query("user_id")
	limitStr := c.Query("limit")
	cursor := c.Query("cursor")

	if roomID == "" || userID == "" {
		c.JSON(400, gin.H{"error": "缺少必要參數"})
		return
	}

	// 解析 limit，從配置讀取默認值
	cfg := config.Get()
	defaultLimit := int32(20)
	if cfg != nil && cfg.Limits.Pagination.DefaultPageSize > 0 {
		defaultLimit = int32(cfg.Limits.Pagination.DefaultPageSize)
	}
	
	limit := defaultLimit
	if limitStr != "" {
		if parsedLimit, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			limit = int32(parsedLimit)
		}
	}

	grpcReq := &chat.GetMessagesRequest{
		RoomId: roomID,
		UserId: userID,
		Limit:  limit,
		Cursor: cursor,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.GetMessages(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success":     resp.Success,
		"message":     resp.Message,
		"data":        resp.Messages,
		"next_cursor": resp.NextCursor,
		"has_more":    resp.HasMore,
	})
}

// 標記消息已讀
func markAsRead(c *gin.Context) {
	var req struct {
		RoomID    string `json:"room_id"`
		UserID    string `json:"user_id"`
		MessageID string `json:"message_id,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	grpcReq := &chat.MarkAsReadRequest{
		RoomId:    req.RoomID,
		UserId:    req.UserID,
		MessageId: req.MessageID,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.MarkAsRead(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}

// 添加群組成員
func addRoomMember(c *gin.Context) {
	roomID := c.Param("room_id")

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	grpcReq := &chat.JoinRoomRequest{
		RoomId: roomID,
		UserId: req.UserID,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.JoinRoom(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}

// 移除群組成員
func removeRoomMember(c *gin.Context) {
	roomID := c.Param("room_id")
	userID := c.Param("user_id")

	grpcReq := &chat.LeaveRoomRequest{
		RoomId: roomID,
		UserId: userID,
	}

	// 調用 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	client := chat.NewChatRoomServiceClient(conn)
	resp, err := client.LeaveRoom(context.Background(), grpcReq)
	if err != nil {
		httputil.InternalServerError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}
