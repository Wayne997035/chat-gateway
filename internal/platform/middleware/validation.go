package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"chat-gateway/internal/constants"
	"chat-gateway/internal/platform/config"

	"github.com/gin-gonic/gin"
)

// ValidationError 驗證錯誤
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateMessageContent 驗證訊息內容
func ValidateMessageContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("訊息內容不能為空")
	}

	cfg := config.Get()
	maxLength := constants.DefaultMaxMessageLength
	if cfg != nil && cfg.Limits.Message.MaxLength > 0 {
		maxLength = cfg.Limits.Message.MaxLength
	}

	if len(content) > maxLength {
		return fmt.Errorf("訊息內容超過最大長度限制 (%d 字符)", maxLength)
	}

	// 防止 NULL 字符注入
	if strings.Contains(content, "\x00") {
		return fmt.Errorf("訊息內容包含非法字符")
	}

	return nil
}

// ValidateRoomName 驗證聊天室名稱
func ValidateRoomName(name string) error {
	trimmed := strings.TrimSpace(name)

	if len(trimmed) < constants.MinRoomNameLength {
		return fmt.Errorf("聊天室名稱不能為空")
	}

	cfg := config.Get()
	maxLength := constants.DefaultMaxRoomNameLength
	if cfg != nil && cfg.Limits.Room.MaxNameLength > 0 {
		maxLength = cfg.Limits.Room.MaxNameLength
	}

	if len(name) > maxLength {
		return fmt.Errorf("聊天室名稱超過最大長度限制 (%d 字符)", maxLength)
	}

	// 防止 NULL 字符注入
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("聊天室名稱包含非法字符")
	}

	return nil
}

// ValidateUserID 驗證用戶 ID 格式
func ValidateUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("用戶 ID 不能為空")
	}

	if len(userID) > constants.MaxUserIDLength {
		return fmt.Errorf("用戶 ID 格式錯誤")
	}

	// 防止 NULL 字符注入和特殊字符
	if strings.ContainsAny(userID, "\x00${}[]") {
		return fmt.Errorf("用戶 ID 包含非法字符")
	}

	return nil
}

// ValidateRoomID 驗證聊天室 ID 格式（MongoDB ObjectID）
func ValidateRoomID(roomID string) error {
	if strings.TrimSpace(roomID) == "" {
		return fmt.Errorf("聊天室 ID 不能為空")
	}

	// MongoDB ObjectID 長度為 24 個十六進制字符
	if len(roomID) != 24 {
		return fmt.Errorf("聊天室 ID 格式錯誤")
	}

	// 只允許十六進制字符
	for _, c := range roomID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("聊天室 ID 格式錯誤")
		}
	}

	return nil
}

// SanitizeInput 消毒輸入（移除危險字符）
func SanitizeInput(input string) string {
	// 移除 NULL 字符
	input = strings.ReplaceAll(input, "\x00", "")

	// 移除控制字符（除了換行和 Tab）
	var result strings.Builder
	for _, r := range input {
		if r >= 32 || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// RequestSizeLimiter 限制請求體大小的中間件
func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("請求體過大，最大允許 %d 字節", maxSize),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
