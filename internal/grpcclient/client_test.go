package grpcclient

import (
	"context"
	"testing"
	"time"

	"chat-gateway/proto/chat"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const testRoomID = "room_123"

// TestGRPCConnection 測試 gRPC 連接
func TestGRPCConnection(t *testing.T) {
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("跳過測試：無法連接到 gRPC 服務器: %v", err)
		return
	}
	defer conn.Close()

	client := chat.NewChatRoomServiceClient(conn)
	if client == nil {
		t.Fatal("創建 gRPC 客戶端失敗")
	}
}

// TestDirectChat 測試一對一聊天
func TestDirectChat(t *testing.T) {
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("跳過測試：無法連接到 gRPC 服務器: %v", err)
		return
	}
	defer conn.Close()

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
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("跳過測試：無法連接到 gRPC 服務器: %v", err)
		return
	}
	defer conn.Close()

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
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("跳過測試：無法連接到 gRPC 服務器: %v", err)
		return
	}
	defer conn.Close()

	client := chat.NewChatRoomServiceClient(conn)

	// 發送消息
	sendReq := &chat.SendMessageRequest{
		RoomId:   testRoomID,
		SenderId: "user_alice",
		Content:  "測試消息 - " + time.Now().Format("15:04:05"),
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
		RoomId: testRoomID,
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

// TestStreamMessages 測試流式消息（需要手動測試）
func TestStreamMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("跳過長時間運行的流式測試")
	}

	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("跳過測試：無法連接到 gRPC 服務器: %v", err)
		return
	}
	defer conn.Close()

	client := chat.NewChatRoomServiceClient(conn)

	req := &chat.StreamMessagesRequest{
		RoomId: testRoomID,
		UserId: "user_alice",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.StreamMessages(ctx, req)
	if err != nil {
		t.Fatalf("開啟流式消息失敗: %v", err)
	}

	t.Log("流式消息連接成功，等待 5 秒...")

	// 嘗試接收一條消息或超時
	done := make(chan bool)
	go func() {
		msg, err := stream.Recv()
		if err == nil {
			t.Logf("收到消息: %s (發送者: %s)", msg.Content, msg.SenderId)
		}
		done <- true
	}()

	select {
	case <-done:
		t.Log("流式測試完成")
	case <-ctx.Done():
		t.Log("流式測試超時（正常）")
	}
}
