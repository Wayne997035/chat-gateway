package server

import (
	"context"
	"io"
	"time"

	"chat-gateway/internal/constants"
	"chat-gateway/internal/grpcclient"
	"chat-gateway/internal/platform/config"
	"chat-gateway/proto/chat"

	"github.com/gin-gonic/gin"
)

// streamMessages 使用 SSE 流式推送訊息
func streamMessages(c *gin.Context) {
	roomID := c.Query("room_id")
	userID := c.Query("user_id")

	if roomID == "" || userID == "" {
		c.JSON(400, gin.H{"error": "缺少 room_id 或 user_id 參數"})
		return
	}

	// 設置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	// CORS 已由全局中間件處理，不需要在這裡設置

	// 發送初始連接確認
	c.SSEvent("connected", gin.H{"status": "ok"})
	c.Writer.Flush()

	// 連接 gRPC 服務
	conn, err := grpcclient.GetConnection()
	if err != nil {
		c.SSEvent("error", gin.H{"message": "連接 gRPC 服務失敗"})
		c.Writer.Flush()
		return
	}

	client := chat.NewChatRoomServiceClient(conn)

	// 建立 gRPC Stream
	stream, err := client.StreamMessages(context.Background(), &chat.StreamMessagesRequest{
		RoomId: roomID,
		UserId: userID,
	})
	if err != nil {
		c.SSEvent("error", gin.H{"message": "建立訊息流失敗: " + err.Error()})
		c.Writer.Flush()
		return
	}

	// 創建一個 channel 來接收訊息
	cfg := config.Get()
	channelBuffer := constants.MessageChannelBuffer
	if cfg != nil && cfg.Limits.SSE.MessageChannelBuffer > 0 {
		channelBuffer = cfg.Limits.SSE.MessageChannelBuffer
	}
	msgChan := make(chan *chat.ChatMessage, channelBuffer)
	errChan := make(chan error, 1)

	// 在 goroutine 中接收 gRPC 訊息
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- msg
		}
	}()

	// 心跳定時器
	heartbeatInterval := 15 // 默認 15 秒
	if cfg != nil && cfg.Limits.SSE.HeartbeatInterval > 0 {
		heartbeatInterval = cfg.Limits.SSE.HeartbeatInterval
	}
	ticker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
	defer ticker.Stop()

	// 持續接收並推送訊息
	for {
		select {
		case <-c.Request.Context().Done():
			// 客戶端斷開連接
			return

		case <-ticker.C:
			// 發送心跳保持連接
			c.SSEvent("ping", gin.H{"timestamp": time.Now().Unix()})
			c.Writer.Flush()

		case msg := <-msgChan:
			// 推送訊息給前端
			c.SSEvent("message", gin.H{
				"id":         msg.Id,
				"room_id":    msg.RoomId,
				"sender_id":  msg.SenderId,
				"content":    msg.Content,
				"type":       msg.Type,
				"created_at": msg.CreatedAt,
				"updated_at": msg.UpdatedAt,
				"read_by":    msg.ReadBy,
			})
			c.Writer.Flush()

		case err := <-errChan:
			if err == io.EOF {
				// 流正常結束
				return
			}
			// 發送錯誤但不立即關閉，讓客戶端決定重連
			c.SSEvent("error", gin.H{"message": "接收訊息失敗: " + err.Error()})
			c.Writer.Flush()
			return
		}
	}
}
