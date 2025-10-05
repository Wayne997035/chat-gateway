package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"
	"unicode/utf8"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/logger"
	"chat-gateway/internal/security/audit"
	"chat-gateway/internal/security/encryption"
	"chat-gateway/internal/security/keymanager"
	"chat-gateway/internal/storage/database"
	"chat-gateway/internal/storage/database/chatroom"
	"chat-gateway/proto/chat"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	systemSenderID         = "system"
	messageFormatErrorText = "[訊息格式錯誤]"
	messageText            = "[訊息]"
	decryptFailedText      = "[解密失敗]"
	roomTypeDirect         = "direct"
)

// Server gRPC 服務器
type Server struct {
	chat.UnimplementedChatRoomServiceServer
	grpcServer *grpc.Server
	repos      *database.Repositories
	encryption *encryption.MessageEncryption
	audit      *audit.AuditService
}

// cleanReadBy 清理和去重 read_by 列表
// 去除重複的用戶ID，並排除發送者本人
func cleanReadBy(readByList []chatroom.MessageReadBy, senderID string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range readByList {
		// 跳過發送者本人
		if item.UserID == senderID {
			continue
		}
		// 去重
		if !seen[item.UserID] {
			seen[item.UserID] = true
			result = append(result, item.UserID)
		}
	}

	return result
}

// isValidUTF8 檢查字符串是否是有效的 UTF-8 編碼
func isValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

// NewServer 創建新的 gRPC 服務器
func NewServer(
	repos *database.Repositories,
	encryptionEnabled, auditEnabled bool,
	keyManager *keymanager.KeyManagerWithPersistence,
	tlsConfig config.TLSConfig,
) (*Server, error) {
	var grpcServer *grpc.Server
	ctx := context.Background()

	// 根據 TLS 配置決定是否啟用 TLS
	if tlsConfig.Enabled {
		tlsCreds, err := loadTLSCredentials(tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		grpcServer = grpc.NewServer(grpc.Creds(tlsCreds))
		logger.Info(ctx, "gRPC TLS 已啟用")
	} else {
		grpcServer = grpc.NewServer()
		logger.Info(ctx, "gRPC 以非加密模式運行（開發環境）")
	}

	server := &Server{
		grpcServer: grpcServer,
		repos:      repos,
		encryption: encryption.NewMessageEncryption(encryptionEnabled, keyManager),
		audit:      audit.NewAuditService(auditEnabled),
	}

	// 註冊服務
	chat.RegisterChatRoomServiceServer(grpcServer, server)

	logger.Infof(ctx, "gRPC 服務器初始化 - 加密: %v, 審計: %v, TLS: %v", encryptionEnabled, auditEnabled, tlsConfig.Enabled)

	// 如果啟用加密，記錄密鑰管理器狀態
	if encryptionEnabled && keyManager != nil {
		stats := keyManager.Stats()
		logger.Infof(ctx, "密鑰管理器已初始化 - 活躍密鑰: %d", stats.ActiveKeys)
	}

	return server, nil
}

// loadTLSCredentials 載入 TLS 憑證
func loadTLSCredentials(tlsConfig config.TLSConfig) (credentials.TransportCredentials, error) {
	// 載入服務器證書和私鑰
	serverCert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	// 創建 TLS 配置
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS12, // 最低 TLS 1.2
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// 如果有 CA 文件，啟用客戶端證書驗證
	if tlsConfig.CAFile != "" {
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to append CA certs")
		}

		config.ClientCAs = certPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(config), nil
}

// Start 啟動 gRPC 服務器
func (s *Server) Start(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	ctx := context.Background()
	logger.Infof(ctx, "gRPC 服務器啟動在端口 %s", port)
	return s.grpcServer.Serve(lis)
}

// Stop 停止 gRPC 服務器
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

// CreateRoom 創建聊天室
func (s *Server) CreateRoom(ctx context.Context, req *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	// 如果是私聊，檢查是否已經存在相同的私聊聊天室
	if req.Type == roomTypeDirect && len(req.MemberIds) == 2 {
		if existingRoom := s.findExistingDirectChat(ctx, req.OwnerId, req.MemberIds); existingRoom != nil {
			logger.Infof(ctx, "找到重複的私聊聊天室: %s", existingRoom.ID)
			return &chat.CreateRoomResponse{
				Success: true,
				Message: "聊天室已存在",
				Room:    convertRoomToGRPC(existingRoom),
			}, nil
		}
	}

	// 確保創建者在成員列表中（如果不在，自動加入）
	memberIds := ensureOwnerInMembers(req.OwnerId, req.MemberIds)

	// 創建房間成員（所有人都是 member，沒有管理員）
	members := createRoomMembers(memberIds)

	// 創建聊天室數據模型
	room := &chatroom.ChatRoom{
		Name:    req.Name,
		Type:    req.Type,
		OwnerID: req.OwnerId,
		Members: members,
		Settings: chatroom.RoomSettings{
			AllowInvite:         req.Settings.AllowInvite,
			AllowEditMessages:   req.Settings.AllowEditMessages,
			AllowDeleteMessages: req.Settings.AllowDeleteMessages,
			AllowPinMessages:    req.Settings.AllowPinMessages,
			MaxMembers:          int(req.Settings.MaxMembers),
			WelcomeMessage:      req.Settings.WelcomeMessage,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到數據庫
	err := s.repos.ChatRoom.Create(ctx, room)
	if err != nil {
		logger.Errorf(ctx, "創建聊天室失敗: %v", err)
		return &chat.CreateRoomResponse{
			Success: false,
			Message: "創建聊天室失敗: " + err.Error(),
		}, nil
	}

	// 審計日誌
	s.audit.LogRoomCreation(ctx, req.OwnerId, room.ID, req.Type)

	logger.Info(ctx, "創建聊天室成功",
		logger.WithRoomID(room.ID),
		logger.WithUserID(req.OwnerId),
		logger.WithAction("create_room"),
		logger.WithDetails(map[string]interface{}{
			"room_name": room.Name,
			"room_type": req.Type,
		}))

	// 轉換為 gRPC 響應格式
	grpcRoom := &chat.ChatRoom{
		Id:      room.ID,
		Name:    room.Name,
		Type:    room.Type,
		OwnerId: room.OwnerID,
		Settings: &chat.RoomSettings{
			AllowInvite:         room.Settings.AllowInvite,
			AllowEditMessages:   room.Settings.AllowEditMessages,
			AllowDeleteMessages: room.Settings.AllowDeleteMessages,
			AllowPinMessages:    room.Settings.AllowPinMessages,
			MaxMembers:          int32(room.Settings.MaxMembers), // #nosec G115 -- MaxMembers is from DB
			WelcomeMessage:      room.Settings.WelcomeMessage,
		},
		CreatedAt: room.CreatedAt.Unix(),
		UpdatedAt: room.UpdatedAt.Unix(),
	}

	// 添加成員信息
	grpcRoom.Members = convertMembersToGRPC(room.Members)

	return &chat.CreateRoomResponse{
		Success: true,
		Message: "聊天室創建成功",
		Room:    grpcRoom,
	}, nil
}

// JoinRoom 加入聊天室
func (s *Server) JoinRoom(ctx context.Context, req *chat.JoinRoomRequest) (*chat.JoinRoomResponse, error) {
	// 檢查成員是否已存在
	isMember, err := s.repos.ChatRoom.IsMember(ctx, req.RoomId, req.UserId)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "檢查成員失敗", req.UserId, req.RoomId, err)
		return &chat.JoinRoomResponse{
			Success: false,
			Message: "檢查成員失敗: " + err.Error(),
		}, nil
	}

	if isMember {
		logger.Info(ctx, "用戶已經是聊天室成員",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId))
		return &chat.JoinRoomResponse{
			Success: true,
			Message: "用戶已經是聊天室成員",
		}, nil
	}

	// 創建成員數據
	member := &chatroom.RoomMember{
		UserID: req.UserId,
		Role:   "member",
	}

	// 添加成員到聊天室
	err = s.repos.ChatRoom.AddMember(ctx, req.RoomId, member)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "加入聊天室失敗", req.UserId, req.RoomId, err)
		return &chat.JoinRoomResponse{
			Success: false,
			Message: "加入聊天室失敗: " + err.Error(),
		}, nil
	}

	// 發送系統消息：XXX 已加入群組
	s.createSystemMessageAndUpdateRoom(ctx, req.RoomId, req.UserId+" 已加入群組", "創建加入群組系統消息失敗")

	// 審計日誌
	s.audit.LogRoomJoin(ctx, req.UserId, req.RoomId)

	logger.Info(ctx, "用戶成功加入聊天室",
		logger.WithUserID(req.UserId),
		logger.WithRoomID(req.RoomId),
		logger.WithAction("join_room"))
	return &chat.JoinRoomResponse{
		Success: true,
		Message: "成功加入聊天室",
	}, nil
}

// LeaveRoom 離開聊天室
func (s *Server) LeaveRoom(ctx context.Context, req *chat.LeaveRoomRequest) (*chat.LeaveRoomResponse, error) {
	// 從聊天室移除成員
	err := s.repos.ChatRoom.RemoveMember(ctx, req.RoomId, req.UserId)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "離開聊天室失敗", req.UserId, req.RoomId, err)
		return &chat.LeaveRoomResponse{
			Success: false,
			Message: "離開聊天室失敗: " + err.Error(),
		}, nil
	}

	// 發送系統消息：XXX 已離開群組
	s.createSystemMessageAndUpdateRoom(ctx, req.RoomId, req.UserId+" 已離開群組", "創建離開群組系統消息失敗")

	// 審計日誌
	s.audit.LogRoomLeave(ctx, req.UserId, req.RoomId)

	logger.Info(ctx, "用戶成功離開聊天室",
		logger.WithUserID(req.UserId),
		logger.WithRoomID(req.RoomId),
		logger.WithAction("leave_room"))
	return &chat.LeaveRoomResponse{
		Success: true,
		Message: "成功離開聊天室",
	}, nil
}

// GetRoomInfo 獲取聊天室信息
func (s *Server) GetRoomInfo(ctx context.Context, req *chat.GetRoomInfoRequest) (*chat.GetRoomInfoResponse, error) {
	// TODO: 實現獲取聊天室信息邏輯
	return &chat.GetRoomInfoResponse{
		Success: true,
		Message: "獲取聊天室信息成功",
		Room: &chat.ChatRoom{
			Id:        req.RoomId,
			Name:      "示例聊天室",
			Type:      "group",
			CreatedAt: 1234567890,
		},
	}, nil
}

// ListUserRooms 列出用戶的聊天室
func (s *Server) ListUserRooms(ctx context.Context, req *chat.ListUserRoomsRequest) (*chat.ListUserRoomsResponse, error) {
	// 設定預設 limit
	cfg := config.Get()
	defaultLimit := 10
	if cfg != nil && cfg.Limits.Pagination.DefaultPageSize > 0 {
		defaultLimit = cfg.Limits.Pagination.DefaultPageSize
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = defaultLimit
	}

	// 從數據庫獲取用戶聊天室（使用 cursor 分頁）
	rooms, cursor, hasMore, err := s.repos.ChatRoom.ListUserRooms(ctx, req.UserId, limit, req.Cursor)
	if err != nil {
		logErrorWithUser(ctx, "獲取用戶聊天室失敗", req.UserId, err)
		return &chat.ListUserRoomsResponse{
			Success: false,
			Message: "獲取聊天室列表失敗: " + err.Error(),
		}, nil
	}

	// 轉換為 gRPC 格式
	grpcRooms := make([]*chat.ChatRoom, len(rooms))
	for i, room := range rooms {
		// 轉換成員信息
		grpcMembers := convertMembersToGRPC(room.Members)

		// 處理最後訊息時間
		var lastMessageTime int64
		if !room.LastMessageTime.IsZero() {
			lastMessageTime = room.LastMessageTime.Unix()
		}

		// 解密 last_message（如果已加密）
		lastMessage := room.LastMessage
		if lastMessage != "" && s.encryption.IsEncrypted(lastMessage) {
			decryptedLastMessage, err := s.encryption.DecryptMessage(lastMessage, room.ID)
			if err != nil {
				logger.Error(ctx, "解密 last_message 失敗",
					logger.WithRoomID(room.ID),
					logger.WithDetails(map[string]interface{}{"error": err.Error()}))
				// 解密失敗，顯示通用訊息
				lastMessage = messageText
			} else {
				// 確保是有效的 UTF-8（防止 gRPC 序列化錯誤）
				if !isValidUTF8(decryptedLastMessage) {
					logger.Warning(ctx, "last_message 包含無效的 UTF-8 字符",
						logger.WithRoomID(room.ID))
					lastMessage = messageText
				} else {
					lastMessage = decryptedLastMessage
				}
			}
		}

		grpcRooms[i] = &chat.ChatRoom{
			Id:              room.ID,
			Name:            room.Name,
			Type:            room.Type,
			OwnerId:         room.OwnerID,
			Members:         grpcMembers,
			CreatedAt:       room.CreatedAt.Unix(),
			UpdatedAt:       room.UpdatedAt.Unix(),
			LastMessage:     lastMessage,
			LastMessageTime: lastMessageTime,
		}
	}

	logger.Info(ctx, "獲取用戶聊天室成功",
		logger.WithUserID(req.UserId),
		logger.WithAction("list_rooms"),
		logger.WithDetails(map[string]interface{}{
			"count":    len(grpcRooms),
			"has_more": hasMore,
			"cursor":   cursor,
		}))

	return &chat.ListUserRoomsResponse{
		Success: true,
		Message: "獲取聊天室列表成功",
		Rooms:   grpcRooms,
		Cursor:  cursor,
		HasMore: hasMore,
	}, nil
}

// SendMessage 發送消息
func (s *Server) SendMessage(ctx context.Context, req *chat.SendMessageRequest) (*chat.SendMessageResponse, error) {
	// 加密並創建消息
	message, encryptedContent, err := s.createEncryptedMessage(ctx, req)
	if err != nil {
		return &chat.SendMessageResponse{Success: false, Message: err.Error()}, nil
	}

	// 更新聊天室最後訊息
	s.updateRoomLastMessage(ctx, req, &message)

	// 審計和日誌
	s.audit.LogMessageSent(ctx, req.SenderId, req.RoomId, message.GetID(), req.Type)
	logger.Info(ctx, "消息發送成功",
		logger.WithUserID(req.SenderId),
		logger.WithRoomID(req.RoomId),
		logger.WithMessageID(message.GetID()),
		logger.WithAction("send_message"),
		logger.WithDetails(map[string]interface{}{
			"encrypted": s.encryption.IsEncrypted(encryptedContent),
			"type":      req.Type,
		}))

	// 構建響應
	grpcMessage := s.buildMessageResponse(ctx, &message)

	return &chat.SendMessageResponse{
		Success:     true,
		Message:     "消息發送成功",
		ChatMessage: grpcMessage,
	}, nil
}

// GetMessages 獲取消息
func (s *Server) GetMessages(ctx context.Context, req *chat.GetMessagesRequest) (*chat.GetMessagesResponse, error) {
	// 從數據庫獲取消息（使用分頁參數）
	messages, nextCursor, hasMore, err := s.repos.Message.GetByRoomID(ctx, req.RoomId, int(req.Limit), req.Cursor, nil, nil)
	if err != nil {
		logErrorWithRoom(ctx, "獲取消息失敗", req.RoomId, err)
		return &chat.GetMessagesResponse{
			Success: false,
			Message: "獲取消息失敗: " + err.Error(),
		}, nil
	}

	// 轉換為 gRPC 格式並解密
	grpcMessages := make([]*chat.ChatMessage, len(messages))
	for i, msg := range messages {
		// 系統訊息不需要解密（純文本）
		decryptedContent := msg.Content
		if msg.Type != systemSenderID {
			// 只解密非系統訊息
			var err error
			decryptedContent, err = s.encryption.DecryptMessage(msg.Content, msg.RoomID)
			if err != nil {
				logger.Warning(ctx, "消息解密失敗",
					logger.WithMessageID(msg.GetID()),
					logger.WithRoomID(msg.RoomID),
					logger.WithDetails(map[string]interface{}{"error": err.Error()}))
				decryptedContent = decryptFailedText
			}
		}

		// 確保內容是有效的 UTF-8（防止 gRPC 序列化錯誤）
		if !isValidUTF8(decryptedContent) {
			logger.Warning(ctx, "消息包含無效的 UTF-8 字符",
				logger.WithMessageID(msg.GetID()),
				logger.WithRoomID(msg.RoomID))
			decryptedContent = messageFormatErrorText
		}

		// 清理並轉換已讀信息（去重、排除發送者）
		grpcReadBy := cleanReadBy(msg.ReadBy, msg.SenderID)

		grpcMessages[i] = &chat.ChatMessage{
			Id:        msg.GetID(),
			RoomId:    msg.RoomID,
			SenderId:  msg.SenderID,
			Content:   decryptedContent, // 返回解密後的內容
			Type:      msg.Type,
			CreatedAt: msg.CreatedAt.Unix(),
			UpdatedAt: msg.UpdatedAt.Unix(),
			ReadBy:    grpcReadBy,
		}
	}

	logger.Info(ctx, "獲取消息成功",
		logger.WithRoomID(req.RoomId),
		logger.WithAction("get_messages"),
		logger.WithDetails(map[string]interface{}{
			"count":   len(grpcMessages),
			"hasMore": hasMore,
			"limit":   req.Limit,
			"cursor":  req.Cursor,
		}))

	return &chat.GetMessagesResponse{
		Success:    true,
		Message:    "獲取消息成功",
		Messages:   grpcMessages,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// StreamMessages 流式獲取消息
func (s *Server) StreamMessages(req *chat.StreamMessagesRequest, stream chat.ChatRoomService_StreamMessagesServer) error {
	ctx := stream.Context()

	logger.Info(ctx, "開始訊息流",
		logger.WithUserID(req.UserId),
		logger.WithRoomID(req.RoomId))

	// 初始化已見訊息集合
	seenMessageIDs := s.initializeSeenMessages(ctx, req.RoomId)

	// 持續監聽新訊息
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "訊息流結束",
				logger.WithUserID(req.UserId),
				logger.WithRoomID(req.RoomId))
			return nil

		case <-ticker.C:
			if err := s.fetchAndStreamNewMessages(ctx, req, stream, seenMessageIDs); err != nil {
				return err
			}
		}
	}
}

// MarkAsRead 標記為已讀
func (s *Server) MarkAsRead(ctx context.Context, req *chat.MarkAsReadRequest) (*chat.MarkAsReadResponse, error) {
	// 標記消息為已讀
	var messageID *string
	if req.MessageId != "" {
		messageID = &req.MessageId
	}
	err := s.repos.Message.MarkAsRead(ctx, req.RoomId, req.UserId, messageID)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "標記已讀失敗", req.UserId, req.RoomId, err)
		return &chat.MarkAsReadResponse{
			Success: false,
			Message: "標記已讀失敗: " + err.Error(),
		}, nil
	}

	// 審計日誌
	msgID := ""
	if req.MessageId != "" {
		msgID = req.MessageId
	}
	s.audit.LogMessageRead(ctx, req.UserId, req.RoomId, msgID)

	logger.Info(ctx, "標記消息已讀成功",
		logger.WithUserID(req.UserId),
		logger.WithRoomID(req.RoomId),
		logger.WithMessageID(msgID),
		logger.WithAction("mark_as_read"))
	return &chat.MarkAsReadResponse{
		Success: true,
		Message: "標記已讀成功",
	}, nil
}

// GetUnreadCount 獲取未讀數量
func (s *Server) GetUnreadCount(ctx context.Context, req *chat.GetUnreadCountRequest) (*chat.GetUnreadCountResponse, error) {
	// 獲取該聊天室的所有訊息
	messages, _, _, err := s.repos.Message.GetByRoomID(ctx, req.RoomId, 1000, "", nil, nil)
	if err != nil {
		logger.Error(ctx, "獲取訊息失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.GetUnreadCountResponse{
			Success: false,
			Message: "獲取未讀數量失敗",
			Count:   0,
		}, nil
	}

	// 計算未讀數量（訊息的 read_by 中不包含該用戶）
	unreadCount := int32(0)
	for _, message := range messages {
		isRead := false
		for _, readBy := range message.ReadBy {
			if readBy.UserID == req.UserId {
				isRead = true
				break
			}
		}
		if !isRead && message.SenderID != req.UserId {
			unreadCount++
		}
	}

	logger.Info(ctx, "獲取未讀數量成功",
		logger.WithUserID(req.UserId),
		logger.WithRoomID(req.RoomId),
		logger.WithDetails(map[string]interface{}{"count": unreadCount}))

	return &chat.GetUnreadCountResponse{
		Success: true,
		Message: "獲取未讀數量成功",
		Count:   unreadCount,
	}, nil
}

// logErrorWithUserAndRoom 記錄包含用戶和聊天室信息的錯誤日誌
func logErrorWithUserAndRoom(ctx context.Context, message, userID, roomID string, err error) {
	logger.Error(ctx, message,
		logger.WithUserID(userID),
		logger.WithRoomID(roomID),
		logger.WithDetails(map[string]interface{}{"error": err.Error()}))
}

// logErrorWithUser 記錄包含用戶信息的錯誤日誌
func logErrorWithUser(ctx context.Context, message, userID string, err error) {
	logger.Error(ctx, message,
		logger.WithUserID(userID),
		logger.WithDetails(map[string]interface{}{"error": err.Error()}))
}

// logErrorWithRoom 記錄包含聊天室信息的錯誤日誌
func logErrorWithRoom(ctx context.Context, message, roomID string, err error) {
	logger.Error(ctx, message,
		logger.WithRoomID(roomID),
		logger.WithDetails(map[string]interface{}{"error": err.Error()}))
}

// convertMembersToGRPC 將成員列表轉換為 gRPC 格式
func convertMembersToGRPC(members []chatroom.RoomMember) []*chat.RoomMember {
	grpcMembers := make([]*chat.RoomMember, len(members))
	for i := range members {
		member := &members[i]
		grpcMembers[i] = &chat.RoomMember{
			UserId:   member.UserID,
			Username: member.Username,
			Role:     member.Role,
			JoinedAt: member.JoinedAt.Unix(),
			LastSeen: member.LastSeen.Unix(),
		}
	}
	return grpcMembers
}

// createSystemMessageAndUpdateRoom 創建系統消息並更新聊天室最後訊息
func (s *Server) createSystemMessageAndUpdateRoom(ctx context.Context, roomID, content, warningPrefix string) {
	systemMessage := chatroom.NewMessage()
	systemMessage.RoomID = roomID
	systemMessage.SenderID = systemSenderID
	systemMessage.Content = content
	systemMessage.Type = systemSenderID

	if err := s.repos.Message.Create(ctx, &systemMessage); err != nil {
		logger.Warning(ctx, warningPrefix,
			logger.WithRoomID(roomID),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
	} else {
		// 更新聊天室的最後訊息
		if updateErr := s.repos.ChatRoom.Update(ctx, roomID, map[string]interface{}{
			"last_message":      systemMessage.Content,
			"last_message_time": systemMessage.CreatedAt,
			"last_message_at":   systemMessage.CreatedAt,
			"updated_at":        systemMessage.CreatedAt,
		}); updateErr != nil {
			logger.Warning(ctx, "更新聊天室最後訊息失敗（"+warningPrefix+"）",
				logger.WithRoomID(roomID),
				logger.WithDetails(map[string]interface{}{"error": updateErr.Error()}))
		}
	}
}

// findExistingDirectChat 查找現有的私聊聊天室
func (s *Server) findExistingDirectChat(ctx context.Context, ownerID string, memberIds []string) *chatroom.ChatRoom {
	cfg := config.Get()
	checkLimit := 100
	if cfg != nil && cfg.Limits.MongoDB.MaxQueryLimit > 0 {
		checkLimit = cfg.Limits.MongoDB.MaxQueryLimit
	}

	existingRooms, _, _, err := s.repos.ChatRoom.ListUserRooms(ctx, ownerID, checkLimit, "")
	if err != nil {
		return nil
	}

	memberIDMap := make(map[string]bool, len(memberIds))
	for _, id := range memberIds {
		memberIDMap[id] = true
	}

	for _, room := range existingRooms {
		if room.Type != roomTypeDirect || len(room.Members) != 2 {
			continue
		}

		// 檢查是否包含相同的兩個成員
		if s.hasSameMembers(room.Members, memberIDMap) {
			return room
		}
	}
	return nil
}

// hasSameMembers 檢查聊天室成員是否與給定的成員ID匹配
func (s *Server) hasSameMembers(members []chatroom.RoomMember, memberIDMap map[string]bool) bool {
	if len(members) != len(memberIDMap) {
		return false
	}
	for i := range members {
		if !memberIDMap[members[i].UserID] {
			return false
		}
	}
	return true
}

// ensureOwnerInMembers 確保創建者在成員列表中
func ensureOwnerInMembers(ownerID string, memberIds []string) []string {
	for _, memberID := range memberIds {
		if memberID == ownerID {
			return memberIds
		}
	}
	return append([]string{ownerID}, memberIds...)
}

// createRoomMembers 創建房間成員列表
func createRoomMembers(memberIds []string) []chatroom.RoomMember {
	now := time.Now()
	members := make([]chatroom.RoomMember, len(memberIds))
	for i, memberID := range memberIds {
		members[i] = chatroom.RoomMember{
			UserID:      memberID,
			Username:    memberID,
			DisplayName: memberID,
			Role:        "member",
			Status:      "active",
			JoinedAt:    now,
			LastSeen:    now,
		}
	}
	return members
}

// convertRoomToGRPC 將聊天室轉換為 gRPC 格式
func convertRoomToGRPC(room *chatroom.ChatRoom) *chat.ChatRoom {
	return &chat.ChatRoom{
		Id:        room.ID,
		Name:      room.Name,
		Type:      room.Type,
		OwnerId:   room.OwnerID,
		Members:   convertMembersToGRPC(room.Members),
		CreatedAt: room.CreatedAt.Unix(),
		UpdatedAt: room.UpdatedAt.Unix(),
	}
}

// createEncryptedMessage 創建並加密消息
func (s *Server) createEncryptedMessage(ctx context.Context, req *chat.SendMessageRequest) (chatroom.Message, string, error) {
	// 加密消息內容
	encryptedContent, err := s.encryption.EncryptMessage(req.Content, req.RoomId)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "消息加密失敗", req.SenderId, req.RoomId, err)
		return chatroom.Message{}, "", fmt.Errorf("消息加密失敗: %w", err)
	}

	// 創建消息數據模型
	message := chatroom.NewMessage()
	message.RoomID = req.RoomId
	message.SenderID = req.SenderId
	message.Content = encryptedContent
	message.Type = req.Type

	// 保存到數據庫
	err = s.repos.Message.Create(ctx, &message)
	if err != nil {
		logErrorWithUserAndRoom(ctx, "發送消息失敗", req.SenderId, req.RoomId, err)
		return chatroom.Message{}, "", fmt.Errorf("發送消息失敗: %w", err)
	}

	return message, encryptedContent, nil
}

// generateLastMessagePreview 生成最後訊息預覽
func generateLastMessagePreview(msgType, content string) string {
	switch msgType {
	case "text":
		if len(content) > 30 {
			runes := []rune(content)
			if len(runes) > 30 {
				return string(runes[:30]) + "..."
			}
			return content
		}
		return content
	case "image":
		return "[圖片]"
	case "file":
		return "[文件]"
	case "audio":
		return "[語音]"
	case "video":
		return "[影片]"
	case "location":
		return "[位置]"
	default:
		return messageText
	}
}

// updateRoomLastMessage 更新聊天室最後訊息
func (s *Server) updateRoomLastMessage(ctx context.Context, req *chat.SendMessageRequest, message *chatroom.Message) {
	// 生成預覽
	lastMessagePreview := generateLastMessagePreview(req.Type, req.Content)

	// 加密 last_message（系統訊息不加密）
	encryptedLastMessage := lastMessagePreview
	if req.Type != systemSenderID {
		encrypted, err := s.encryption.EncryptMessage(lastMessagePreview, req.RoomId)
		if err != nil {
			logger.Error(ctx, "加密 last_message 失敗",
				logger.WithRoomID(req.RoomId),
				logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			// 降級處理，使用明文
		} else {
			encryptedLastMessage = encrypted
		}
	}

	// 更新聊天室
	err := s.repos.ChatRoom.Update(ctx, req.RoomId, map[string]interface{}{
		"last_message":      encryptedLastMessage,
		"last_message_time": message.CreatedAt,
		"last_message_at":   message.CreatedAt,
		"updated_at":        message.CreatedAt,
	})
	if err != nil {
		logger.Error(ctx, "更新聊天室最後訊息失敗",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
	}
}

// buildMessageResponse 構建消息響應
func (s *Server) buildMessageResponse(ctx context.Context, message *chatroom.Message) *chat.ChatMessage {
	// 清理已讀信息
	grpcReadBy := cleanReadBy(message.ReadBy, message.SenderID)

	// 解密內容
	responseContent := message.Content
	if message.Type != systemSenderID {
		decrypted, err := s.encryption.DecryptMessage(message.Content, message.RoomID)
		if err != nil {
			logger.Warning(ctx, "返回消息時解密失敗",
				logger.WithMessageID(message.GetID()),
				logger.WithRoomID(message.RoomID),
				logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			responseContent = decryptFailedText
		} else {
			responseContent = decrypted
		}
	}

	return &chat.ChatMessage{
		Id:        message.GetID(),
		RoomId:    message.RoomID,
		SenderId:  message.SenderID,
		Content:   responseContent,
		Type:      message.Type,
		CreatedAt: message.CreatedAt.Unix(),
		UpdatedAt: message.UpdatedAt.Unix(),
		ReadBy:    grpcReadBy,
	}
}

// initializeSeenMessages 初始化已見訊息集合
func (s *Server) initializeSeenMessages(ctx context.Context, roomID string) map[string]bool {
	seenMessageIDs := make(map[string]bool)

	cfg := config.Get()
	initialFetchLimit := 100
	if cfg != nil && cfg.Limits.SSE.InitialMessageFetch > 0 {
		initialFetchLimit = cfg.Limits.SSE.InitialMessageFetch
	}

	existingMessages, _, _, err := s.repos.Message.GetByRoomID(
		ctx, roomID, initialFetchLimit, "", nil, nil,
	)
	if err == nil {
		for _, msg := range existingMessages {
			seenMessageIDs[msg.GetID()] = true
		}
		logger.Info(ctx, "初始化訊息流，標記現有訊息",
			logger.WithRoomID(roomID),
			logger.WithDetails(map[string]interface{}{"existingCount": len(existingMessages)}))
	}

	return seenMessageIDs
}

// fetchAndStreamNewMessages 獲取並推送新訊息
func (s *Server) fetchAndStreamNewMessages(
	ctx context.Context,
	req *chat.StreamMessagesRequest,
	stream chat.ChatRoomService_StreamMessagesServer,
	seenMessageIDs map[string]bool,
) error {
	messages, _, _, err := s.repos.Message.GetByRoomID(
		ctx, req.RoomId, 100, "", nil, nil,
	)
	if err != nil {
		logger.Error(ctx, "獲取新訊息失敗",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return nil // 不中斷流，繼續重試
	}

	newMessageCount := 0
	for _, msg := range messages {
		if seenMessageIDs[msg.GetID()] {
			continue
		}

		seenMessageIDs[msg.GetID()] = true
		newMessageCount++

		if err := s.processAndSendMessage(ctx, msg, req.RoomId, stream); err != nil {
			return err
		}
	}

	if newMessageCount > 0 {
		logger.Info(ctx, "推送新訊息",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"count": newMessageCount}))
	}

	return nil
}

// processAndSendMessage 處理並發送單個訊息
func (s *Server) processAndSendMessage(
	ctx context.Context,
	msg *chatroom.Message,
	roomID string,
	stream chat.ChatRoomService_StreamMessagesServer,
) error {
	msgID := msg.GetID()

	// 解密內容
	decryptedContent := msg.Content
	if msg.Type != systemSenderID {
		var err error
		decryptedContent, err = s.encryption.DecryptMessage(msg.Content, roomID)
		if err != nil {
			logger.Error(ctx, "解密訊息失敗",
				logger.WithMessageID(msgID),
				logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			decryptedContent = decryptFailedText
		}
	}

	// 確保內容是有效的 UTF-8
	if !isValidUTF8(decryptedContent) {
		logger.Warning(ctx, "SSE 推送的消息包含無效的 UTF-8 字符",
			logger.WithMessageID(msgID),
			logger.WithRoomID(roomID))
		decryptedContent = messageFormatErrorText
	}

	// 轉換 read_by
	grpcReadBy := make([]string, len(msg.ReadBy))
	for i, readBy := range msg.ReadBy {
		grpcReadBy[i] = readBy.UserID
	}

	// 構建並推送訊息
	grpcMsg := &chat.ChatMessage{
		Id:        msgID,
		RoomId:    msg.RoomID,
		SenderId:  msg.SenderID,
		Content:   decryptedContent,
		Type:      msg.Type,
		CreatedAt: msg.CreatedAt.Unix(),
		UpdatedAt: msg.UpdatedAt.Unix(),
		ReadBy:    grpcReadBy,
	}

	if err := stream.Send(grpcMsg); err != nil {
		logger.Error(ctx, "推送訊息失敗",
			logger.WithMessageID(msgID),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return err
	}

	return nil
}
