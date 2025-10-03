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
func NewServer(repos *database.Repositories, encryptionEnabled bool, auditEnabled bool, keyManager *keymanager.KeyManagerWithPersistence, tlsConfig config.TLSConfig) (*Server, error) {
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
	if req.Type == "direct" && len(req.MemberIds) == 2 {
		cfg := config.Get()
		checkLimit := 100 // 默認
		if cfg != nil && cfg.Limits.MongoDB.MaxQueryLimit > 0 {
			checkLimit = cfg.Limits.MongoDB.MaxQueryLimit
		}
		existingRooms, _, _, err := s.repos.ChatRoom.ListUserRooms(ctx, req.OwnerId, checkLimit, "")
		if err == nil {
			for _, existingRoom := range existingRooms {
				if existingRoom.Type == "direct" && len(existingRoom.Members) == 2 {
					// 檢查是否包含相同的兩個成員
					existingMemberIds := make(map[string]bool)
					for _, member := range existingRoom.Members {
						existingMemberIds[member.UserID] = true
					}

					// 檢查請求中的成員是否都在現有聊天室中
					allMembersExist := true
					for _, memberID := range req.MemberIds {
						if !existingMemberIds[memberID] {
							allMembersExist = false
							break
						}
					}

					if allMembersExist {
						// 找到重複的私聊，返回現有的聊天室
						logger.Infof(ctx, "找到重複的私聊聊天室: %s", existingRoom.ID)

						// 轉換為 gRPC 格式
						grpcMembers := make([]*chat.RoomMember, len(existingRoom.Members))
						for i, member := range existingRoom.Members {
							grpcMembers[i] = &chat.RoomMember{
								UserId:   member.UserID,
								Username: member.Username,
								Role:     member.Role,
								JoinedAt: member.JoinedAt.Unix(),
								LastSeen: member.LastSeen.Unix(),
							}
						}

						grpcRoom := &chat.ChatRoom{
							Id:        existingRoom.ID,
							Name:      existingRoom.Name,
							Type:      existingRoom.Type,
							OwnerId:   existingRoom.OwnerID,
							Members:   grpcMembers,
							CreatedAt: existingRoom.CreatedAt.Unix(),
							UpdatedAt: existingRoom.UpdatedAt.Unix(),
						}

						return &chat.CreateRoomResponse{
							Success: true,
							Message: "聊天室已存在",
							Room:    grpcRoom,
						}, nil
					}
				}
			}
		}
	}

	// 確保創建者在成員列表中（如果不在，自動加入）
	memberIds := req.MemberIds
	ownerInMembers := false
	for _, memberID := range memberIds {
		if memberID == req.OwnerId {
			ownerInMembers = true
			break
		}
	}

	if !ownerInMembers {
		// 創建者不在成員列表中，自動加入
		memberIds = append([]string{req.OwnerId}, memberIds...)
	}

	// 創建房間成員（所有人都是 member，沒有管理員）
	members := make([]chatroom.RoomMember, len(memberIds))
	for i, memberID := range memberIds {
		members[i] = chatroom.RoomMember{
			UserID:      memberID,
			Username:    memberID,
			DisplayName: memberID,
			Role:        "member",
			Status:      "active",
			JoinedAt:    time.Now(),
			LastSeen:    time.Now(),
		}
	}

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
			MaxMembers:          int32(room.Settings.MaxMembers),
			WelcomeMessage:      room.Settings.WelcomeMessage,
		},
		CreatedAt: room.CreatedAt.Unix(),
		UpdatedAt: room.UpdatedAt.Unix(),
	}

	// 添加成員信息
	grpcMembers := make([]*chat.RoomMember, len(room.Members))
	for i, member := range room.Members {
		grpcMembers[i] = &chat.RoomMember{
			UserId:   member.UserID,
			Username: member.Username,
			Role:     member.Role,
			JoinedAt: member.JoinedAt.Unix(),
			LastSeen: member.LastSeen.Unix(),
		}
	}
	grpcRoom.Members = grpcMembers

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
		logger.Error(ctx, "檢查成員失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
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
		logger.Error(ctx, "加入聊天室失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.JoinRoomResponse{
			Success: false,
			Message: "加入聊天室失敗: " + err.Error(),
		}, nil
	}

	// 發送系統消息：XXX 已加入群組
	systemMessage := chatroom.NewMessage()
	systemMessage.RoomID = req.RoomId
	systemMessage.SenderID = "system"
	systemMessage.Content = req.UserId + " 已加入群組"
	systemMessage.Type = "system"

	if err := s.repos.Message.Create(ctx, &systemMessage); err != nil {
		logger.Warning(ctx, "創建加入群組系統消息失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
	} else {
		// 更新聊天室的最後訊息
		s.repos.ChatRoom.Update(ctx, req.RoomId, map[string]interface{}{
			"last_message":      systemMessage.Content,
			"last_message_time": systemMessage.CreatedAt,
			"last_message_at":   systemMessage.CreatedAt,
			"updated_at":        systemMessage.CreatedAt,
		})
	}

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
		logger.Error(ctx, "離開聊天室失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.LeaveRoomResponse{
			Success: false,
			Message: "離開聊天室失敗: " + err.Error(),
		}, nil
	}

	// 發送系統消息：XXX 已離開群組
	systemMessage := chatroom.NewMessage()
	systemMessage.RoomID = req.RoomId
	systemMessage.SenderID = "system"
	systemMessage.Content = req.UserId + " 已離開群組"
	systemMessage.Type = "system"

	if err := s.repos.Message.Create(ctx, &systemMessage); err != nil {
		logger.Warning(ctx, "創建離開群組系統消息失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
	} else {
		// 更新聊天室的最後訊息
		s.repos.ChatRoom.Update(ctx, req.RoomId, map[string]interface{}{
			"last_message":      systemMessage.Content,
			"last_message_time": systemMessage.CreatedAt,
			"last_message_at":   systemMessage.CreatedAt,
			"updated_at":        systemMessage.CreatedAt,
		})
	}

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
		logger.Error(ctx, "獲取用戶聊天室失敗",
			logger.WithUserID(req.UserId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.ListUserRoomsResponse{
			Success: false,
			Message: "獲取聊天室列表失敗: " + err.Error(),
		}, nil
	}

	// 轉換為 gRPC 格式
	grpcRooms := make([]*chat.ChatRoom, len(rooms))
	for i, room := range rooms {
		// 轉換成員信息
		grpcMembers := make([]*chat.RoomMember, len(room.Members))
		for j, member := range room.Members {
			grpcMembers[j] = &chat.RoomMember{
				UserId:   member.UserID,
				Username: member.Username,
				Role:     member.Role,
				JoinedAt: member.JoinedAt.Unix(),
				LastSeen: member.LastSeen.Unix(),
			}
		}

		// 處理最後訊息時間
		var lastMessageTime int64
		if !room.LastMessageTime.IsZero() {
			lastMessageTime = room.LastMessageTime.Unix()
		}

		grpcRooms[i] = &chat.ChatRoom{
			Id:              room.ID,
			Name:            room.Name,
			Type:            room.Type,
			OwnerId:         room.OwnerID,
			Members:         grpcMembers,
			CreatedAt:       room.CreatedAt.Unix(),
			UpdatedAt:       room.UpdatedAt.Unix(),
			LastMessage:     room.LastMessage,
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
	// 加密消息內容
	encryptedContent, err := s.encryption.EncryptMessage(req.Content, req.RoomId)
	if err != nil {
		logger.Error(ctx, "消息加密失敗",
			logger.WithUserID(req.SenderId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.SendMessageResponse{
			Success: false,
			Message: "消息加密失敗: " + err.Error(),
		}, nil
	}

	// 創建消息數據模型
	message := chatroom.NewMessage()
	message.RoomID = req.RoomId
	message.SenderID = req.SenderId
	message.Content = encryptedContent // 存儲加密後的內容
	message.Type = req.Type

	// 保存到數據庫
	err = s.repos.Message.Create(ctx, &message)
	if err != nil {
		logger.Error(ctx, "發送消息失敗",
			logger.WithUserID(req.SenderId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return &chat.SendMessageResponse{
			Success: false,
			Message: "發送消息失敗: " + err.Error(),
		}, nil
	}

	// 更新聊天室的最後訊息
	// 為了 UX，存儲明文預覽（用戶需要在列表中看到內容）
	var lastMessagePreview string
	switch req.Type {
	case "text":
		// 文字訊息：顯示前 30 個字符
		if len(req.Content) > 30 {
			// 確保不會在 UTF-8 字符中間截斷
			runes := []rune(req.Content)
			if len(runes) > 30 {
				lastMessagePreview = string(runes[:30]) + "..."
			} else {
				lastMessagePreview = req.Content
			}
		} else {
			lastMessagePreview = req.Content
		}
	case "image":
		lastMessagePreview = "[圖片]"
	case "file":
		lastMessagePreview = "[文件]"
	case "audio":
		lastMessagePreview = "[語音]"
	case "video":
		lastMessagePreview = "[影片]"
	case "location":
		lastMessagePreview = "[位置]"
	default:
		lastMessagePreview = "[訊息]"
	}

	err = s.repos.ChatRoom.Update(ctx, req.RoomId, map[string]interface{}{
		"last_message":      lastMessagePreview,
		"last_message_time": message.CreatedAt,
		"last_message_at":   message.CreatedAt, // 用於排序的時間戳
		"updated_at":        message.CreatedAt,
	})
	if err != nil {
		logger.Error(ctx, "更新聊天室最後訊息失敗",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
	}

	// 審計日誌
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

	// 清理並轉換已讀信息（去重、排除發送者）
	grpcReadBy := cleanReadBy(message.ReadBy, message.SenderID)

	// 轉換為 gRPC 格式
	grpcMessage := &chat.ChatMessage{
		Id:        message.GetID(),
		RoomId:    message.RoomID,
		SenderId:  message.SenderID,
		Content:   message.Content,
		Type:      message.Type,
		CreatedAt: message.CreatedAt.Unix(),
		UpdatedAt: message.UpdatedAt.Unix(),
		ReadBy:    grpcReadBy,
	}

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
		logger.Error(ctx, "獲取消息失敗",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
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
		if msg.Type != "system" {
			// 只解密非系統訊息
			var err error
			decryptedContent, err = s.encryption.DecryptMessage(msg.Content, msg.RoomID)
			if err != nil {
				logger.Warning(ctx, "消息解密失敗",
					logger.WithMessageID(msg.GetID()),
					logger.WithRoomID(msg.RoomID),
					logger.WithDetails(map[string]interface{}{"error": err.Error()}))
				decryptedContent = "[解密失敗]"
			}
		}

		// 確保內容是有效的 UTF-8（防止 gRPC 序列化錯誤）
		if !isValidUTF8(decryptedContent) {
			logger.Warning(ctx, "消息包含無效的 UTF-8 字符",
				logger.WithMessageID(msg.GetID()),
				logger.WithRoomID(msg.RoomID))
			decryptedContent = "[訊息格式錯誤]"
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

	// 記錄已經推送過的訊息ID（避免重複推送）
	seenMessageIDs := make(map[string]bool)

	// 初始化：獲取現有訊息並標記為已見（不推送歷史訊息）
	cfg := config.Get()
	initialFetchLimit := 100 // 默認
	if cfg != nil && cfg.Limits.SSE.InitialMessageFetch > 0 {
		initialFetchLimit = cfg.Limits.SSE.InitialMessageFetch
	}

	existingMessages, _, _, err := s.repos.Message.GetByRoomID(
		ctx,
		req.RoomId,
		initialFetchLimit,
		"",
		nil,
		nil,
	)
	if err == nil {
		for _, msg := range existingMessages {
			seenMessageIDs[msg.GetID()] = true
		}
		logger.Info(ctx, "初始化訊息流，標記現有訊息",
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"existingCount": len(existingMessages)}))
	}

	// 持續監聽新訊息
	ticker := time.NewTicker(2 * time.Second) // 每2秒檢查一次新訊息
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 客戶端斷開連接
			logger.Info(ctx, "訊息流結束",
				logger.WithUserID(req.UserId),
				logger.WithRoomID(req.RoomId))
			return nil

		case <-ticker.C:
			// 獲取所有訊息
			messages, _, _, err := s.repos.Message.GetByRoomID(
				ctx,
				req.RoomId,
				100, // 一次最多獲取 100 條
				"",
				nil,
				nil,
			)
			if err != nil {
				logger.Error(ctx, "獲取新訊息失敗",
					logger.WithRoomID(req.RoomId),
					logger.WithDetails(map[string]interface{}{"error": err.Error()}))
				continue
			}

			// 只推送新訊息（未見過的訊息）
			newMessageCount := 0
			for _, msg := range messages {
				msgID := msg.GetID()

				// 跳過已經推送過的訊息
				if seenMessageIDs[msgID] {
					continue
				}

				// 標記為已見
				seenMessageIDs[msgID] = true
				newMessageCount++

				// 系統訊息不需要解密（純文本）
				decryptedContent := msg.Content
				if msg.Type != "system" {
					// 只解密非系統訊息
					var err error
					decryptedContent, err = s.encryption.DecryptMessage(msg.Content, req.RoomId)
					if err != nil {
						logger.Error(ctx, "解密訊息失敗",
							logger.WithMessageID(msgID),
							logger.WithDetails(map[string]interface{}{"error": err.Error()}))
						decryptedContent = "[解密失敗]"
					}
				}

				// 確保內容是有效的 UTF-8（防止 gRPC 序列化錯誤）
				if !isValidUTF8(decryptedContent) {
					logger.Warning(ctx, "SSE 推送的消息包含無效的 UTF-8 字符",
						logger.WithMessageID(msgID),
						logger.WithRoomID(req.RoomId))
					decryptedContent = "[訊息格式錯誤]"
				}

				// 轉換 read_by
				grpcReadBy := make([]string, len(msg.ReadBy))
				for i, readBy := range msg.ReadBy {
					grpcReadBy[i] = readBy.UserID
				}

				// 構建 gRPC 訊息
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

				// 推送訊息
				if err := stream.Send(grpcMsg); err != nil {
					logger.Error(ctx, "推送訊息失敗",
						logger.WithMessageID(msgID),
						logger.WithDetails(map[string]interface{}{"error": err.Error()}))
					return err
				}
			}

			// 記錄推送的新訊息數量
			if newMessageCount > 0 {
				logger.Info(ctx, "推送新訊息",
					logger.WithRoomID(req.RoomId),
					logger.WithDetails(map[string]interface{}{"count": newMessageCount}))
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
		logger.Error(ctx, "標記已讀失敗",
			logger.WithUserID(req.UserId),
			logger.WithRoomID(req.RoomId),
			logger.WithDetails(map[string]interface{}{"error": err.Error()}))
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
