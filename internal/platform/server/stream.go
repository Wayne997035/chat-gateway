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
	roomID, userID, ok := validateStreamParams(c)
	if !ok {
		return
	}

	setupSSEHeaders(c)

	stream, ok := createGRPCStream(c, roomID, userID)
	if !ok {
		return
	}

	msgChan, errChan := setupMessageChannels(stream)
	handleSSELoop(c, msgChan, errChan)
}

// validateStreamParams 驗證流參數
func validateStreamParams(c *gin.Context) (roomID, userID string, ok bool) {
	roomID = c.Query("room_id")
	userID = c.Query("user_id")

	if roomID == "" || userID == "" {
		c.JSON(400, gin.H{"error": "缺少 room_id 或 user_id 參數"})
		return "", "", false
	}

	return roomID, userID, true
}

// setupSSEHeaders 設置 SSE headers
func setupSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.SSEvent("connected", gin.H{"status": "ok"})
	c.Writer.Flush()
}

// createGRPCStream 創建 gRPC stream
func createGRPCStream(c *gin.Context, roomID, userID string) (chat.ChatRoomService_StreamMessagesClient, bool) {
	conn, err := grpcclient.GetConnection()
	if err != nil {
		c.SSEvent("error", gin.H{"message": "連接 gRPC 服務失敗"})
		c.Writer.Flush()
		return nil, false
	}

	client := chat.NewChatRoomServiceClient(conn)
	stream, err := client.StreamMessages(context.Background(), &chat.StreamMessagesRequest{
		RoomId: roomID,
		UserId: userID,
	})
	if err != nil {
		c.SSEvent("error", gin.H{"message": "建立訊息流失敗: " + err.Error()})
		c.Writer.Flush()
		return nil, false
	}

	return stream, true
}

// setupMessageChannels 設置訊息通道
func setupMessageChannels(stream chat.ChatRoomService_StreamMessagesClient) (msgChan chan *chat.ChatMessage, errChan chan error) {
	cfg := config.Get()
	channelBuffer := constants.MessageChannelBuffer
	if cfg != nil && cfg.Limits.SSE.MessageChannelBuffer > 0 {
		channelBuffer = cfg.Limits.SSE.MessageChannelBuffer
	}

	msgChan = make(chan *chat.ChatMessage, channelBuffer)
	errChan = make(chan error, 1)

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

	return msgChan, errChan
}

// handleSSELoop 處理 SSE 循環
func handleSSELoop(c *gin.Context, msgChan chan *chat.ChatMessage, errChan chan error) {
	cfg := config.Get()
	heartbeatInterval := 15
	if cfg != nil && cfg.Limits.SSE.HeartbeatInterval > 0 {
		heartbeatInterval = cfg.Limits.SSE.HeartbeatInterval
	}

	ticker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return

		case <-ticker.C:
			c.SSEvent("ping", gin.H{"timestamp": time.Now().Unix()})
			c.Writer.Flush()

		case msg := <-msgChan:
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
				return
			}
			c.SSEvent("error", gin.H{"message": "接收訊息失敗: " + err.Error()})
			c.Writer.Flush()
			return
		}
	}
}
