package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"chat-gateway/proto/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 連接到 gRPC 服務器
	conn, err := grpc.Dial("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("連接失敗: %v", err)
	}
	defer conn.Close()

	// 創建客戶端
	client := chat.NewChatRoomServiceClient(conn)

	fmt.Println("=== gRPC 聊天室測試客戶端 ===")
	fmt.Println()

	// 測試一對一聊天
	testDirectChat(client)

	fmt.Println()

	// 測試群組聊天
	testGroupChat(client)

	fmt.Println()

	// 測試消息功能
	testMessages(client)
}

// 測試一對一聊天
func testDirectChat(client chat.ChatRoomServiceClient) {
	fmt.Println("=== 測試一對一聊天 ===")

	// 創建一對一聊天室
	req := &chat.CreateRoomRequest{
		Name:        "Alice 和 Bob 的私聊",
		Description: "私人對話",
		Type:        "direct",
		OwnerId:     "user_alice",
		MemberIds:   []string{"user_alice", "user_bob"},
		Settings: &chat.RoomSettings{
			AllowInvite:         false,
			AllowEditMessages:   true,
			AllowDeleteMessages: true,
			MaxMembers:          2,
		},
	}

	resp, err := client.CreateRoom(context.Background(), req)
	if err != nil {
		log.Printf("創建一對一聊天室失敗: %v", err)
		return
	}

	fmt.Printf("✓ 一對一聊天室創建成功\n")
	fmt.Printf("  房間 ID: %s\n", resp.Room.Id)
	fmt.Printf("  房間名稱: %s\n", resp.Room.Name)
	fmt.Printf("  房間類型: %s\n", resp.Room.Type)
	fmt.Printf("  擁有者: %s\n", resp.Room.OwnerId)
}

// 測試群組聊天
func testGroupChat(client chat.ChatRoomServiceClient) {
	fmt.Println("=== 測試群組聊天 ===")

	// 創建群組聊天室
	req := &chat.CreateRoomRequest{
		Name:        "開發團隊討論組",
		Description: "技術討論",
		Type:        "group",
		OwnerId:     "user_alice",
		MemberIds:   []string{"user_alice", "user_bob", "user_charlie", "user_david"},
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
		log.Printf("創建群組聊天室失敗: %v", err)
		return
	}

	fmt.Printf("✓ 群組聊天室創建成功\n")
	fmt.Printf("  房間 ID: %s\n", resp.Room.Id)
	fmt.Printf("  房間名稱: %s\n", resp.Room.Name)
	fmt.Printf("  房間類型: %s\n", resp.Room.Type)
	fmt.Printf("  擁有者: %s\n", resp.Room.OwnerId)
	fmt.Printf("  成員數量: %d\n", len(resp.Room.Members))
}

// 測試消息功能
func testMessages(client chat.ChatRoomServiceClient) {
	fmt.Println("=== 測試消息功能 ===")

	roomId := "room_123" // 使用測試房間 ID

	// 發送消息
	sendReq := &chat.SendMessageRequest{
		RoomId:   roomId,
		SenderId: "user_alice",
		Content:  "你好！這是 gRPC 測試消息",
		Type:     "text",
	}

	sendResp, err := client.SendMessage(context.Background(), sendReq)
	if err != nil {
		log.Printf("發送消息失敗: %v", err)
		return
	}

	fmt.Printf("✓ 消息發送成功\n")
	fmt.Printf("  消息 ID: %s\n", sendResp.ChatMessage.Id)
	fmt.Printf("  發送者: %s\n", sendResp.ChatMessage.SenderId)
	fmt.Printf("  內容: %s\n", sendResp.ChatMessage.Content)
	fmt.Printf("  時間: %s\n", time.Unix(sendResp.ChatMessage.CreatedAt, 0).Format("2006-01-02 15:04:05"))

	// 獲取消息
	getReq := &chat.GetMessagesRequest{
		RoomId: roomId,
		UserId: "user_alice",
		Limit:  10,
	}

	getResp, err := client.GetMessages(context.Background(), getReq)
	if err != nil {
		log.Printf("獲取消息失敗: %v", err)
		return
	}

	fmt.Printf("✓ 獲取消息成功\n")
	fmt.Printf("  消息數量: %d\n", len(getResp.Messages))
	for i, msg := range getResp.Messages {
		fmt.Printf("  消息 %d: %s (發送者: %s)\n", i+1, msg.Content, msg.SenderId)
	}
}

// 測試流式消息
func testStreamMessages(client chat.ChatRoomServiceClient) {
	fmt.Println("=== 測試流式消息 ===")

	roomId := "room_123"

	req := &chat.StreamMessagesRequest{
		RoomId: roomId,
		UserId: "user_alice",
	}

	stream, err := client.StreamMessages(context.Background(), req)
	if err != nil {
		log.Printf("開啟流式消息失敗: %v", err)
		return
	}

	fmt.Printf("✓ 流式消息連接成功\n")
	fmt.Printf("  正在監聽房間: %s\n", roomId)
	fmt.Printf("  按 Ctrl+C 停止監聽\n")

	// 監聽消息
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("接收消息失敗: %v", err)
			break
		}

		fmt.Printf("收到新消息: %s (發送者: %s)\n", msg.Content, msg.SenderId)
	}
}
