package chatroom

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CreateIndexes 創建數據庫索引以優化查詢性能
func CreateIndexes(ctx context.Context, db *mongo.Database) error {
	// 消息集合索引
	messagesCollection := db.Collection("messages")
	
	// 1. 聊天室 ID + 創建時間複合索引（最重要的索引）
	roomTimeIndex := mongo.IndexModel{
		Keys: bson.D{
			{"room_id", 1},
			{"created_at", -1},
		},
		Options: options.Index().SetName("room_time_idx"),
	}

	// 2. 發送者 ID + 創建時間索引
	senderTimeIndex := mongo.IndexModel{
		Keys: bson.D{
			{"sender_id", 1},
			{"created_at", -1},
		},
		Options: options.Index().SetName("sender_time_idx"),
	}

	// 3. 消息類型索引
	messageTypeIndex := mongo.IndexModel{
		Keys: bson.D{
			{"type", 1},
		},
		Options: options.Index().SetName("type_idx"),
	}

	// 4. 全文搜索索引
	textSearchIndex := mongo.IndexModel{
		Keys: bson.D{
			{"content", "text"},
		},
		Options: options.Index().SetName("content_text_idx"),
	}

	// 5. 已讀狀態索引
	readStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{"read_by.user_id", 1},
			{"created_at", -1},
		},
		Options: options.Index().SetName("read_status_idx"),
	}

	// 創建消息索引
	messageIndexes := []mongo.IndexModel{
		roomTimeIndex,
		senderTimeIndex,
		messageTypeIndex,
		textSearchIndex,
		readStatusIndex,
	}

	_, err := messagesCollection.Indexes().CreateMany(ctx, messageIndexes)
	if err != nil {
		return err
	}

	// 聊天室集合索引
	chatRoomsCollection := db.Collection("chat_rooms")

	// 1. 聊天室類型索引
	roomTypeIndex := mongo.IndexModel{
		Keys: bson.D{
			{"type", 1},
		},
		Options: options.Index().SetName("room_type_idx"),
	}

	// 2. 擁有者 ID 索引
	ownerIndex := mongo.IndexModel{
		Keys: bson.D{
			{"owner_id", 1},
		},
		Options: options.Index().SetName("owner_idx"),
	}

	// 3. 成員用戶 ID 索引
	memberIndex := mongo.IndexModel{
		Keys: bson.D{
			{"members.user_id", 1},
		},
		Options: options.Index().SetName("member_idx"),
	}

	// 4. 最後消息時間索引
	lastMessageIndex := mongo.IndexModel{
		Keys: bson.D{
			{"last_message_at", -1},
		},
		Options: options.Index().SetName("last_message_idx"),
	}

	// 5. 創建時間索引
	createdAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{"created_at", -1},
		},
		Options: options.Index().SetName("created_at_idx"),
	}

	// 創建聊天室索引
	roomIndexes := []mongo.IndexModel{
		roomTypeIndex,
		ownerIndex,
		memberIndex,
		lastMessageIndex,
		createdAtIndex,
	}

	_, err = chatRoomsCollection.Indexes().CreateMany(ctx, roomIndexes)
	if err != nil {
		return err
	}

	return nil
}

// GetIndexStats 獲取索引統計信息
func GetIndexStats(ctx context.Context, db *mongo.Database) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 消息集合統計
	messagesCollection := db.Collection("messages")
	messagesStats, err := messagesCollection.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}

	var messageIndexes []bson.M
	if err = messagesStats.All(ctx, &messageIndexes); err != nil {
		return nil, err
	}
	stats["messages_indexes"] = messageIndexes

	// 聊天室集合統計
	chatRoomsCollection := db.Collection("chat_rooms")
	roomsStats, err := chatRoomsCollection.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}

	var roomIndexes []bson.M
	if err = roomsStats.All(ctx, &roomIndexes); err != nil {
		return nil, err
	}
	stats["chat_rooms_indexes"] = roomIndexes

	return stats, nil
}
