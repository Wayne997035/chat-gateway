package database

import (
	"context"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/storage/database/chatroom"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Repositories 倉儲集合.
type Repositories struct {
	ChatRoom *chatroom.ChatRoomStore
	Message  *chatroom.MessageStore
}

// NewRepositories 創建倉儲集合.
func NewRepositories(cfg *config.Config) *Repositories {
	// 從 driver 包獲取 MongoDB 連接
	db := mongoDB
	if db == nil {
		// 如果沒有全局 db，嘗試從 driver 獲取
		// 這裡可以根據需要添加連接邏輯
		return nil
	}

	// 創建索引以優化查詢性能
	ctx := context.Background()
	if err := chatroom.CreateIndexes(ctx, db); err != nil {
		// 索引創建失敗不影響服務啟動，繼續運行
		_ = err // 忽略索引創建錯誤
	}

	return &Repositories{
		ChatRoom: chatroom.NewChatRoomStore(db),
		Message:  chatroom.NewMessageStore(db),
	}
}

// 全局變數，用於存儲 MongoDB 連接
var mongoDB *mongo.Database

// SetMongoDB 設置 MongoDB 連接.
func SetMongoDB(db *mongo.Database) {
	mongoDB = db
}
