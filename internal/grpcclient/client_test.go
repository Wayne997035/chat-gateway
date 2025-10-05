package grpcclient

import (
	"context"
	"io"
	"net"
	"testing"

	"chat-gateway/proto/chat"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

// mockChatRoomService 實作 mock gRPC server
type mockChatRoomService struct {
	chat.UnimplementedChatRoomServiceServer
}

func (s *mockChatRoomService) CreateRoom(ctx context.Context, req *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	members := make([]*chat.RoomMember, len(req.MemberIds))
	for i, memberID := range req.MemberIds {
		members[i] = &chat.RoomMember{
			UserId: memberID,
			Role:   "member",
		}
	}

	return &chat.CreateRoomResponse{
		Success: true,
		Message: "聊天室創建成功",
		Room: &chat.ChatRoom{
			Id:        "mock_room_123",
			Name:      req.Name,
			Type:      req.Type,
			OwnerId:   req.OwnerId,
			Members:   members,
			CreatedAt: 1234567890,
			UpdatedAt: 1234567890,
		},
	}, nil
}

func (s *mockChatRoomService) SendMessage(ctx context.Context, req *chat.SendMessageRequest) (*chat.SendMessageResponse, error) {
	return &chat.SendMessageResponse{
		Success: true,
		Message: "消息發送成功",
		ChatMessage: &chat.ChatMessage{
			Id:        "mock_msg_123",
			RoomId:    req.RoomId,
			SenderId:  req.SenderId,
			Content:   req.Content,
			Type:      req.Type,
			CreatedAt: 1234567890,
			UpdatedAt: 1234567890,
			ReadBy:    []string{},
		},
	}, nil
}

func (s *mockChatRoomService) GetMessages(ctx context.Context, req *chat.GetMessagesRequest) (*chat.GetMessagesResponse, error) {
	messages := []*chat.ChatMessage{
		{
			Id:        "mock_msg_1",
			RoomId:    req.RoomId,
			SenderId:  "user_alice",
			Content:   "測試消息 1",
			Type:      "text",
			CreatedAt: 1234567890,
			UpdatedAt: 1234567890,
			ReadBy:    []string{},
		},
	}

	return &chat.GetMessagesResponse{
		Success:  true,
		Message:  "獲取消息成功",
		Messages: messages,
		HasMore:  false,
	}, nil
}

func (s *mockChatRoomService) StreamMessages(req *chat.StreamMessagesRequest, stream chat.ChatRoomService_StreamMessagesServer) error {
	// 模擬發送一條消息
	msg := &chat.ChatMessage{
		Id:        "mock_stream_msg",
		RoomId:    req.RoomId,
		SenderId:  "user_bob",
		Content:   "流式消息測試",
		Type:      "text",
		CreatedAt: 1234567890,
		UpdatedAt: 1234567890,
		ReadBy:    []string{},
	}

	if err := stream.Send(msg); err != nil {
		return err
	}

	return nil
}

// bufDialer 用於測試的內存連接
func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

// setupMockServer 設置 mock gRPC server
func setupMockServer(t *testing.T) *grpc.ClientConn {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	chat.RegisterChatRoomServiceServer(s, &mockChatRoomService{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		s.Stop()
		lis.Close()
	})

	return conn
}

// TestGRPCConnection 測試 gRPC 連接
func TestGRPCConnection(t *testing.T) {
	conn := setupMockServer(t)
	client := chat.NewChatRoomServiceClient(conn)

	if client == nil {
		t.Fatal("創建 gRPC 客戶端失敗")
	}
}

// TestDirectChat 測試一對一聊天
func TestDirectChat(t *testing.T) {
	conn := setupMockServer(t)
	client := chat.NewChatRoomServiceClient(conn)

	req := &chat.CreateRoomRequest{
		Name:      "Alice 和 Bob 的私聊",
		Type:      "direct",
		OwnerId:   "user_alice",
		MemberIds: []string{"user_alice", "user_bob"},
		Settings: &chat.RoomSettings{
			AllowInvite:         false,
			AllowEditMessages:   true,
			AllowDeleteMessages: true,
			MaxMembers:          2,
		},
	}

	resp, err := client.CreateRoom(context.Background(), req)
	if err != nil {
		t.Fatalf("創建一對一聊天室失敗: %v", err)
	}

	if resp.Room == nil {
		t.Fatal("返回的房間為 nil")
	}

	if resp.Room.Type != "direct" {
		t.Errorf("期望房間類型為 'direct'，實際為 '%s'", resp.Room.Type)
	}

	t.Logf("一對一聊天室創建成功: %s", resp.Room.Id)
}

// TestGroupChat 測試群組聊天
func TestGroupChat(t *testing.T) {
	conn := setupMockServer(t)
	client := chat.NewChatRoomServiceClient(conn)

	req := &chat.CreateRoomRequest{
		Name:      "開發團隊討論組",
		Type:      "group",
		OwnerId:   "user_alice",
		MemberIds: []string{"user_alice", "user_bob", "user_charlie", "user_david"},
		Settings: &chat.RoomSettings{
			AllowInvite:         true,
			AllowEditMessages:   true,
			AllowDeleteMessages: true,
			MaxMembers:          100,
			WelcomeMessage:      "歡迎加入開發團隊討論組！",
		},
	}

	resp, err := client.CreateRoom(context.Background(), req)
	if err != nil {
		t.Fatalf("創建群組聊天室失敗: %v", err)
	}

	if resp.Room == nil {
		t.Fatal("返回的房間為 nil")
	}

	if resp.Room.Type != "group" {
		t.Errorf("期望房間類型為 'group'，實際為 '%s'", resp.Room.Type)
	}

	if len(resp.Room.Members) != 4 {
		t.Errorf("期望成員數量為 4，實際為 %d", len(resp.Room.Members))
	}

	t.Logf("群組聊天室創建成功: %s", resp.Room.Id)
}

// TestSendAndGetMessages 測試發送和獲取消息
func TestSendAndGetMessages(t *testing.T) {
	conn := setupMockServer(t)
	client := chat.NewChatRoomServiceClient(conn)

	// 發送消息
	sendReq := &chat.SendMessageRequest{
		RoomId:   "test_room",
		SenderId: "user_alice",
		Content:  "測試消息",
		Type:     "text",
	}

	sendResp, err := client.SendMessage(context.Background(), sendReq)
	if err != nil {
		t.Fatalf("發送消息失敗: %v", err)
	}

	if sendResp.ChatMessage == nil {
		t.Fatal("返回的消息為 nil")
	}

	messageID := sendResp.ChatMessage.Id
	t.Logf("消息發送成功: %s", messageID)

	// 獲取消息
	getReq := &chat.GetMessagesRequest{
		RoomId: "test_room",
		UserId: "user_alice",
		Limit:  10,
	}

	getResp, err := client.GetMessages(context.Background(), getReq)
	if err != nil {
		t.Fatalf("獲取消息失敗: %v", err)
	}

	if len(getResp.Messages) == 0 {
		t.Error("期望至少有一條消息")
	}

	t.Logf("獲取消息成功，數量: %d", len(getResp.Messages))
}

// TestStreamMessages 測試流式消息
func TestStreamMessages(t *testing.T) {
	conn := setupMockServer(t)
	client := chat.NewChatRoomServiceClient(conn)

	req := &chat.StreamMessagesRequest{
		RoomId: "test_room",
		UserId: "user_alice",
	}

	stream, err := client.StreamMessages(context.Background(), req)
	if err != nil {
		t.Fatalf("開啟流式消息失敗: %v", err)
	}

	// 接收一條消息
	msg, err := stream.Recv()
	if err != nil && err != io.EOF {
		t.Fatalf("接收消息失敗: %v", err)
	}

	if msg != nil {
		t.Logf("收到流式消息: %s (發送者: %s)", msg.Content, msg.SenderId)
	}
}
