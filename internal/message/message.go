package message

import (
	"net/http"

	"chat-gateway/internal/httputil"
	"chat-gateway/internal/storage/database/message"

	"github.com/gin-gonic/gin"
)

// MessageHandler message 處理器.
type MessageHandler struct {
	messageRepo *message.MessageStore
}

// NewMessageHandler 創建新的 message 處理器.
func NewMessageHandler(messageRepo *message.MessageStore) *MessageHandler {
	return &MessageHandler{
		messageRepo: messageRepo,
	}
}

// CreateMessage 創建消息.
func (h *MessageHandler) CreateMessage(c *gin.Context) {
	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httputil.ErrorMessage("Invalid request format"))
		return
	}

	// 驗證請求
	if err := ValidateCreateMessageRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, httputil.ErrorMessage(err.Error()))
		return
	}

	// 創建消息記錄
	msg := &message.Message{
		Content:   req.Content,
		SenderID:  req.SenderID,
		ChannelID: req.ChannelID,
		Type:      req.Type,
		Status:    "sent",
	}

	if err := h.messageRepo.Create(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, httputil.ErrorMessage("Failed to create message"))
		return
	}

	c.JSON(http.StatusCreated, httputil.NewSuccessResponse("Message created successfully", msg))
}

// GetMessage 獲取消息.
func (h *MessageHandler) GetMessage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, httputil.ErrorMessage("Message ID is required"))
		return
	}

	msg, err := h.messageRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, httputil.ErrorMessage("Message not found"))
		return
	}

	c.JSON(http.StatusOK, httputil.NewSuccessResponse("Message retrieved successfully", msg))
}

// ListMessages 列出消息.
func (h *MessageHandler) ListMessages(c *gin.Context) {
	channelID := c.Query("channel_id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, httputil.ErrorMessage("Channel ID is required"))
		return
	}

	filter := map[string]interface{}{
		"channel_id": channelID,
	}

	// 解析分頁參數
	limit := int64(20) // 預設限制
	offset := int64(0) // 預設偏移

	messages, err := h.messageRepo.List(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httputil.ErrorMessage("Failed to retrieve messages"))
		return
	}

	c.JSON(http.StatusOK, httputil.NewSuccessResponse("Messages retrieved successfully", messages))
}
