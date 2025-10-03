package audit

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// AuditService 審計服務
type AuditService struct {
	enabled bool
	logger  *log.Logger
}

// NewAuditService 創建審計服務
func NewAuditService(enabled bool) *AuditService {
	return &AuditService{
		enabled: enabled,
		logger:  log.Default(),
	}
}

// AuditEvent 審計事件
type AuditEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id"`
	RoomID      string                 `json:"room_id,omitempty"`
	MessageID   string                 `json:"message_id,omitempty"`
	Action      string                 `json:"action"`
	Result      string                 `json:"result"` // success, failure
	Details     map[string]interface{} `json:"details,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
}

// LogRoomCreation 記錄聊天室創建
func (a *AuditService) LogRoomCreation(ctx context.Context, userID, roomID, roomType string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "room_creation",
		UserID:    userID,
		RoomID:    roomID,
		Action:    "create_room",
		Result:    "success",
		Details: map[string]interface{}{
			"room_type": roomType,
		},
	}

	a.enrichWithMetadata(ctx, &event)
	a.log(event)
}

// LogMessageSent 記錄消息發送
func (a *AuditService) LogMessageSent(ctx context.Context, userID, roomID, messageID, messageType string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "message_sent",
		UserID:    userID,
		RoomID:    roomID,
		MessageID: messageID,
		Action:    "send_message",
		Result:    "success",
		Details: map[string]interface{}{
			"message_type": messageType,
		},
	}

	a.log(event)
}

// LogMessageRead 記錄消息已讀
func (a *AuditService) LogMessageRead(ctx context.Context, userID, roomID, messageID string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "message_read",
		UserID:    userID,
		RoomID:    roomID,
		MessageID: messageID,
		Action:    "mark_as_read",
		Result:    "success",
	}

	a.log(event)
}

// LogRoomJoin 記錄加入聊天室
func (a *AuditService) LogRoomJoin(ctx context.Context, userID, roomID string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "room_join",
		UserID:    userID,
		RoomID:    roomID,
		Action:    "join_room",
		Result:    "success",
	}

	a.log(event)
}

// LogRoomLeave 記錄離開聊天室
func (a *AuditService) LogRoomLeave(ctx context.Context, userID, roomID string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "room_leave",
		UserID:    userID,
		RoomID:    roomID,
		Action:    "leave_room",
		Result:    "success",
	}

	a.log(event)
}

// LogMemberAdded 記錄添加成員
func (a *AuditService) LogMemberAdded(ctx context.Context, operatorID, roomID, memberID string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "member_added",
		UserID:    operatorID,
		RoomID:    roomID,
		Action:    "add_member",
		Result:    "success",
		Details: map[string]interface{}{
			"member_id": memberID,
		},
	}

	a.log(event)
}

// LogMemberRemoved 記錄移除成員
func (a *AuditService) LogMemberRemoved(ctx context.Context, operatorID, roomID, memberID string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "member_removed",
		UserID:    operatorID,
		RoomID:    roomID,
		Action:    "remove_member",
		Result:    "success",
		Details: map[string]interface{}{
			"member_id": memberID,
		},
	}

	a.log(event)
}

// LogAuthenticationFailure 記錄認證失敗
func (a *AuditService) LogAuthenticationFailure(ctx context.Context, userID, reason string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "authentication",
		UserID:    userID,
		Action:    "authenticate",
		Result:    "failure",
		Details: map[string]interface{}{
			"reason": reason,
		},
	}

	a.log(event)
}

// LogRateLimitExceeded 記錄速率限制超過
func (a *AuditService) LogRateLimitExceeded(ctx context.Context, ipAddress, endpoint string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "rate_limit",
		Action:    "api_request",
		Result:    "blocked",
		IPAddress: ipAddress,
		Details: map[string]interface{}{
			"endpoint": endpoint,
			"reason":   "rate_limit_exceeded",
		},
	}

	a.log(event)
}

// LogSuspiciousActivity 記錄可疑活動
func (a *AuditService) LogSuspiciousActivity(ctx context.Context, userID, ipAddress, activityType, description string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "suspicious_activity",
		UserID:    userID,
		Action:    activityType,
		Result:    "flagged",
		IPAddress: ipAddress,
		Details: map[string]interface{}{
			"description": description,
		},
	}

	a.log(event)
}

// LogAccessDenied 記錄訪問被拒絕
func (a *AuditService) LogAccessDenied(ctx context.Context, userID, roomID, reason string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "access_denied",
		UserID:    userID,
		RoomID:    roomID,
		Action:    "access_resource",
		Result:    "denied",
		Details: map[string]interface{}{
			"reason": reason,
		},
	}

	a.log(event)
}

// LogDataModification 記錄數據修改
func (a *AuditService) LogDataModification(ctx context.Context, userID, resourceType, resourceID, operation string, changes map[string]interface{}) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "data_modification",
		UserID:    userID,
		Action:    operation,
		Result:    "success",
		Details: map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"changes":       changes,
		},
	}

	a.log(event)
}

// LogSecurityEvent 記錄安全事件
func (a *AuditService) LogSecurityEvent(ctx context.Context, eventType, description string, severity string, details map[string]interface{}) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "security_event",
		Action:    eventType,
		Result:    severity,
		Details: map[string]interface{}{
			"description": description,
			"severity":    severity,
			"details":     details,
		},
	}

	a.log(event)
}

// log 記錄審計事件
func (a *AuditService) log(event AuditEvent) {
	// 轉換為 JSON
	jsonData, err := json.Marshal(event)
	if err != nil {
		a.logger.Printf("[AUDIT-ERROR] Failed to marshal event: %v", err)
		return
	}

	// 記錄到日誌
	a.logger.Printf("[AUDIT] %s", string(jsonData))

	// TODO: 同時寫入專門的審計日誌文件或數據庫
	// 1. 寫入 MongoDB 審計集合
	// 2. 或寫入專門的審計日誌文件
	// 3. 或發送到 SIEM 系統
}

// IsEnabled 檢查審計是否啟用
func (a *AuditService) IsEnabled() bool {
	return a.enabled
}

// enrichWithMetadata 從 context 提取元數據並豐富審計事件
func (a *AuditService) enrichWithMetadata(ctx context.Context, event *AuditEvent) {
	// 定義 context key（需要與 middleware 一致）
	type contextKey string
	const requestMetadataKey contextKey = "request_metadata"
	
	// 嘗試從 context 提取元數據
	if metadata := ctx.Value(requestMetadataKey); metadata != nil {
		if meta, ok := metadata.(*struct {
			IPAddress string
			UserAgent string
			UserID    string
		}); ok {
			event.IPAddress = meta.IPAddress
			event.UserAgent = meta.UserAgent
		}
	}
}

