package main

import (
	"fmt"
	"log"

	"chat-gateway/internal/chatroom"
	"chat-gateway/internal/storage/database/chatroom"
)

func main() {
	fmt.Println("Chat Room Service Test")
	
	// 測試聊天室功能
	testChatRoom()
	
	// 測試消息功能
	testMessage()
}

func testChatRoom() {
	fmt.Println("\n=== Testing Chat Room ===")
	
	// 創建聊天室請求
	req := chatroom.CreateRoomRequest{
		Name:        "測試聊天室",
		Description: "這是一個測試聊天室",
		Type:        "group",
		OwnerID:     "user_123",
		MemberIDs:   []string{"user_456", "user_789"},
		Settings: chatroom.RoomSettings{
			AllowInvite:         true,
			AllowEditMessages:   true,
			AllowDeleteMessages: true,
			AllowPinMessages:    true,
			MaxMembers:          100,
			WelcomeMessage:      "歡迎來到測試聊天室！",
		},
	}
	
	// 驗證請求
	if err := chatroom.ValidateCreateRoomRequest(&req); err != nil {
		log.Printf("Validation error: %v", err)
	} else {
		fmt.Println("✓ Chat room request validation passed")
	}
	
	// 測試默認設置
	defaultSettings := chatroom.GetDefaultRoomSettings()
	fmt.Printf("✓ Default room settings: %+v\n", defaultSettings)
}

func testMessage() {
	fmt.Println("\n=== Testing Message ===")
	
	// 創建消息請求
	req := chatroom.SendMessageRequest{
		RoomID:   "room_123",
		SenderID: "user_123",
		Content:  "Hello, World!",
		Type:     "text",
		Metadata: chatroom.CreateTextMessageMetadata(),
	}
	
	// 驗證請求
	if err := chatroom.ValidateSendMessageRequest(&req); err != nil {
		log.Printf("Validation error: %v", err)
	} else {
		fmt.Println("✓ Message request validation passed")
	}
	
	// 測試不同類型的消息元數據
	imageMetadata := chatroom.CreateImageMessageMetadata(
		"https://example.com/image.jpg",
		"https://example.com/thumb.jpg",
		800,
		600,
	)
	fmt.Printf("✓ Image message metadata: %+v\n", imageMetadata)
	
	fileMetadata := chatroom.CreateFileMessageMetadata(
		"document.pdf",
		"application/pdf",
		"1024KB",
		"https://example.com/document.pdf",
	)
	fmt.Printf("✓ File message metadata: %+v\n", fileMetadata)
	
	locationMetadata := chatroom.CreateLocationMessageMetadata(
		25.0330,
		121.5654,
		"台北101",
	)
	fmt.Printf("✓ Location message metadata: %+v\n", locationMetadata)
}

func testWebSocket() {
	fmt.Println("\n=== Testing WebSocket ===")
	
	// 這裡可以添加 WebSocket 測試
	fmt.Println("✓ WebSocket tests would go here")
}
