package message

import (
	"errors"
	"strings"
)

// CreateMessageRequest 創建消息請求.
type CreateMessageRequest struct {
	Content   string `json:"content" binding:"required"`
	SenderID  string `json:"sender_id" binding:"required"`
	ChannelID string `json:"channel_id" binding:"required"`
	Type      string `json:"type" binding:"required"`
}

// UpdateMessageRequest 更新消息請求.
type UpdateMessageRequest struct {
	Content string `json:"content,omitempty"`
	Status  string `json:"status,omitempty"`
}

// MessageListRequest 消息列表請求.
type MessageListRequest struct {
	ChannelID string `form:"channel_id" binding:"required"`
	Limit     int    `form:"limit"`
	Offset    int    `form:"offset"`
}

// 消息類型常數.
const (
	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeFile  = "file"
	MessageTypeAudio = "audio"
	MessageTypeVideo = "video"
)

// 消息狀態常數.
const (
	MessageStatusSent     = "sent"
	MessageStatusDelivered = "delivered"
	MessageStatusRead     = "read"
	MessageStatusFailed   = "failed"
)
