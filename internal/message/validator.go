package message

import (
	"errors"
	"strings"
)

// ValidateCreateMessageRequest 驗證創建消息請求.
func ValidateCreateMessageRequest(req *CreateMessageRequest) error {
	if strings.TrimSpace(req.Content) == "" {
		return errors.New("content cannot be empty")
	}

	if strings.TrimSpace(req.SenderID) == "" {
		return errors.New("sender_id cannot be empty")
	}

	if strings.TrimSpace(req.ChannelID) == "" {
		return errors.New("channel_id cannot be empty")
	}

	if !isValidMessageType(req.Type) {
		return errors.New("invalid message type")
	}

	return nil
}

// ValidateUpdateMessageRequest 驗證更新消息請求.
func ValidateUpdateMessageRequest(req *UpdateMessageRequest) error {
	if req.Content != "" && strings.TrimSpace(req.Content) == "" {
		return errors.New("content cannot be empty")
	}

	if req.Status != "" && !isValidMessageStatus(req.Status) {
		return errors.New("invalid message status")
	}

	return nil
}

// isValidMessageType 檢查消息類型是否有效.
func isValidMessageType(messageType string) bool {
	validTypes := []string{
		MessageTypeText,
		MessageTypeImage,
		MessageTypeFile,
		MessageTypeAudio,
		MessageTypeVideo,
	}

	for _, validType := range validTypes {
		if messageType == validType {
			return true
		}
	}

	return false
}

// isValidMessageStatus 檢查消息狀態是否有效.
func isValidMessageStatus(status string) bool {
	validStatuses := []string{
		MessageStatusSent,
		MessageStatusDelivered,
		MessageStatusRead,
		MessageStatusFailed,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}

	return false
}
