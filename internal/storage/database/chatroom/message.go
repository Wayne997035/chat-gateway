package chatroom

import (
	"context"
	"time"

	"chat-gateway/internal/platform/config"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MessageRepository 消息倉儲接口
type MessageRepository interface {
	Create(ctx context.Context, message *Message) error
	GetByID(ctx context.Context, id string) (*Message, error)
	GetByRoomID(ctx context.Context, roomID string, limit int, cursor string, since, until *time.Time) ([]*Message, string, bool, error)
	Update(ctx context.Context, id string, update map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	MarkAsRead(ctx context.Context, roomID, userID string, messageID *string) error
	MarkAsDelivered(ctx context.Context, roomID, userID string, messageID *string) error
	GetUnreadCount(ctx context.Context, userID string, roomID *string) (int, error)
	Search(ctx context.Context, roomID, query string, userID *string, messageType *string, since, until *time.Time, limit int, cursor string) ([]*Message, string, bool, int, error)
}

// Message 消息數據模型
type Message struct {
	_ID              interface{}            `bson:"_id" form:"_id"`
	ID               string                 `json:"id,omitempty" bson:"id" form:"id"`
	RoomID           string                 `bson:"room_id" json:"room_id"`
	SenderID         string                 `bson:"sender_id" json:"sender_id"`
	Content          string                 `bson:"content" json:"content"`
	Type             string                 `bson:"type" json:"type"`
	Status           string                 `bson:"status" json:"status"`
	CreatedAt        time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `bson:"updated_at" json:"updated_at"`
	DeliveredAt      *time.Time             `bson:"delivered_at,omitempty" json:"delivered_at,omitempty"`
	ReadAt           *time.Time             `bson:"read_at,omitempty" json:"read_at,omitempty"`
	Metadata         MessageMetadata        `bson:"metadata" json:"metadata"`
	EncryptionKeyID  string                 `bson:"encryption_key_id,omitempty" json:"encryption_key_id,omitempty"`
	EncryptedContent string                 `bson:"encrypted_content,omitempty" json:"encrypted_content,omitempty"`
	Signature        string                 `bson:"signature,omitempty" json:"signature,omitempty"`
	ReplyToMessageID string                 `bson:"reply_to_message_id,omitempty" json:"reply_to_message_id,omitempty"`
	ForwardedFrom    []string               `bson:"forwarded_from,omitempty" json:"forwarded_from,omitempty"`
	ReadBy           []MessageReadBy        `bson:"read_by,omitempty" json:"read_by,omitempty"`
	DeliveredTo      []MessageDeliveredTo   `bson:"delivered_to,omitempty" json:"delivered_to,omitempty"`
	CustomData       map[string]interface{} `bson:"custom_data,omitempty" json:"custom_data,omitempty"`
}

// GetID 獲取 ID 的字符串形式
func (m *Message) GetID() string {
	return m.ID
}

// NewMessage 創建新的 Message 實例
func NewMessage() Message {
	_id := bson.NewObjectID()
	now := time.Now().UTC()
	return Message{_ID: _id, ID: _id.Hex(), CreatedAt: now, UpdatedAt: now}
}

// MessageMetadata 消息元數據
type MessageMetadata struct {
	FileName       string  `bson:"file_name,omitempty" json:"file_name,omitempty"`
	FileSize       string  `bson:"file_size,omitempty" json:"file_size,omitempty"`
	FileType       string  `bson:"file_type,omitempty" json:"file_type,omitempty"`
	FileURL        string  `bson:"file_url,omitempty" json:"file_url,omitempty"`
	ImageURL       string  `bson:"image_url,omitempty" json:"image_url,omitempty"`
	ImageThumbnail string  `bson:"image_thumbnail,omitempty" json:"image_thumbnail,omitempty"`
	ImageWidth     int32   `bson:"image_width,omitempty" json:"image_width,omitempty"`
	ImageHeight    int32   `bson:"image_height,omitempty" json:"image_height,omitempty"`
	Latitude       float64 `bson:"latitude,omitempty" json:"latitude,omitempty"`
	Longitude      float64 `bson:"longitude,omitempty" json:"longitude,omitempty"`
	LocationName   string  `bson:"location_name,omitempty" json:"location_name,omitempty"`
}

// MessageReadBy 消息已讀記錄
type MessageReadBy struct {
	UserID    string    `bson:"user_id" json:"user_id"`
	ReadAt    time.Time `bson:"read_at" json:"read_at"`
	IPAddress string    `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
}

// MessageDeliveredTo 消息已送達記錄
type MessageDeliveredTo struct {
	UserID      string    `bson:"user_id" json:"user_id"`
	DeliveredAt time.Time `bson:"delivered_at" json:"delivered_at"`
	IPAddress   string    `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
}

// MessageStore 消息存儲實作
type MessageStore struct {
	collection *mongo.Collection
}

// NewMessageStore 創建新的消息存儲
func NewMessageStore(db *mongo.Database) *MessageStore {
	return &MessageStore{
		collection: db.Collection("messages"),
	}
}

// Create 創建消息
func (s *MessageStore) Create(ctx context.Context, message *Message) error {
	_id := bson.NewObjectID()
	message._ID = _id
	message.ID = _id.Hex()
	message.CreatedAt = time.Now()
	message.UpdatedAt = time.Now()
	message.Status = "sent"

	// 初始化已讀和送達列表
	if message.ReadBy == nil {
		message.ReadBy = []MessageReadBy{}
	}
	if message.DeliveredTo == nil {
		message.DeliveredTo = []MessageDeliveredTo{}
	}

	_, err := s.collection.InsertOne(ctx, message)
	return err
}

// GetByID 根據 ID 獲取消息
func (s *MessageStore) GetByID(ctx context.Context, id string) (*Message, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var message Message
	err = s.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&message)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

// GetByRoomID 根據聊天室 ID 獲取消息
func (s *MessageStore) GetByRoomID(ctx context.Context, roomID string, limit int, cursor string, since, until *time.Time) ([]*Message, string, bool, error) {
	// 從配置讀取限制
	cfg := config.Get()
	defaultLimit := 20
	maxLimit := 100
	if cfg != nil {
		if cfg.Limits.Pagination.DefaultPageSize > 0 {
			defaultLimit = cfg.Limits.Pagination.DefaultPageSize
		}
		if cfg.Limits.MongoDB.MaxQueryLimit > 0 {
			maxLimit = cfg.Limits.MongoDB.MaxQueryLimit
		}
	}

	// 限制分頁大小，防止性能問題
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	filter := bson.M{"room_id": roomID}

	// 添加時間範圍過濾
	if since != nil {
		filter["created_at"] = bson.M{"$gte": *since}
	}
	if until != nil {
		if filter["created_at"] == nil {
			filter["created_at"] = bson.M{"$lte": *until}
		} else {
			filter["created_at"].(bson.M)["$lte"] = *until
		}
	}

	// 如果有游標，添加游標條件（查找比游標時間更早的訊息）
	if cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			filter["created_at"] = bson.M{"$lt": cursorTime}
		}
	}

	opts := options.Find()
	opts.SetLimit(int64(limit + 1))                      // 多取一個用於判斷是否有更多
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // 按創建時間倒序排列（新消息在前）
	opts.SetProjection(bson.M{                           // 只選擇需要的字段，減少網絡傳輸
		"_id":                 1,
		"id":                  1,
		"room_id":             1,
		"sender_id":           1,
		"content":             1,
		"type":                1,
		"status":              1,
		"created_at":          1,
		"updated_at":          1,
		"read_by":             1,
		"delivered_to":        1,
		"metadata":            1,
		"reply_to_message_id": 1,
		"forwarded_from":      1,
	})

	cursorResult, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", false, err
	}
	defer cursorResult.Close(ctx)

	var messages []*Message
	for cursorResult.Next(ctx) {
		var message Message
		if err := cursorResult.Decode(&message); err != nil {
			return nil, "", false, err
		}
		messages = append(messages, &message)
	}

	// 檢查是否有更多數據
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit] // 移除多取的那一個
	}

	// 生成下一個游標
	var nextCursor string
	if hasMore && len(messages) > 0 {
		nextCursor = messages[len(messages)-1].CreatedAt.Format(time.RFC3339)
	}

	return messages, nextCursor, hasMore, nil
}

// GetHistoryMessages 獲取歷史消息（優化版本）
func (s *MessageStore) GetHistoryMessages(ctx context.Context, roomID string, limit int, cursor string) ([]*Message, string, bool, error) {
	// 從配置讀取限制
	cfg := config.Get()
	defaultLimit := 20
	maxHistoryLimit := 50 // 歷史消息限制更嚴格
	if cfg != nil {
		if cfg.Limits.Pagination.DefaultPageSize > 0 {
			defaultLimit = cfg.Limits.Pagination.DefaultPageSize
		}
		if cfg.Limits.Pagination.MaxHistorySize > 0 {
			maxHistoryLimit = cfg.Limits.Pagination.MaxHistorySize
		}
	}

	// 嚴格限制分頁大小
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	filter := bson.M{
		"room_id": roomID,
		"type":    bson.M{"$ne": "system"}, // 排除系統消息
	}

	opts := options.Find()
	opts.SetLimit(int64(limit + 1))
	opts.SetSort(bson.D{{Key: "created_at", Value: 1}}) // 按創建時間正序排列（舊消息在上，新消息在下）

	// 只選擇必要字段，提高查詢性能
	opts.SetProjection(bson.M{
		"_id":                 1,
		"id":                  1,
		"room_id":             1,
		"sender_id":           1,
		"content":             1,
		"type":                1,
		"status":              1,
		"created_at":          1,
		"metadata":            1,
		"reply_to_message_id": 1,
	})

	// 處理游標
	if cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			filter["created_at"] = bson.M{"$lt": cursorTime}
		}
	}

	cursorResult, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", false, err
	}
	defer cursorResult.Close(ctx)

	var messages []*Message
	for cursorResult.Next(ctx) {
		var message Message
		if err := cursorResult.Decode(&message); err != nil {
			return nil, "", false, err
		}
		messages = append(messages, &message)
	}

	// 檢查是否有更多數據
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	// 生成下一個游標
	var nextCursor string
	if hasMore && len(messages) > 0 {
		nextCursor = messages[len(messages)-1].CreatedAt.Format(time.RFC3339)
	}

	return messages, nextCursor, hasMore, nil
}

// GetRecentMessages 獲取最近消息（用於實時更新）
func (s *MessageStore) GetRecentMessages(ctx context.Context, roomID string, since time.Time, limit int) ([]*Message, error) {
	// 從配置讀取限制
	cfg := config.Get()
	defaultLimit := 20
	maxLimit := 100
	if cfg != nil {
		if cfg.Limits.Pagination.DefaultPageSize > 0 {
			defaultLimit = cfg.Limits.Pagination.DefaultPageSize
		}
		if cfg.Limits.MongoDB.MaxQueryLimit > 0 {
			maxLimit = cfg.Limits.MongoDB.MaxQueryLimit
		}
	}

	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	filter := bson.M{
		"room_id":    roomID,
		"created_at": bson.M{"$gt": since},
	}

	opts := options.Find()
	opts.SetLimit(int64(limit))
	opts.SetSort(bson.D{{Key: "created_at", Value: 1}}) // 按時間正序排列

	cursorResult, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursorResult.Close(ctx)

	var messages []*Message
	for cursorResult.Next(ctx) {
		var message Message
		if err := cursorResult.Decode(&message); err != nil {
			return nil, err
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

// Update 更新消息
func (s *MessageStore) Update(ctx context.Context, id string, update map[string]interface{}) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	update["updated_at"] = time.Now()
	_, err = s.collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	return err
}

// Delete 刪除消息
func (s *MessageStore) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	return err
}

// MarkAsRead 標記消息為已讀
func (s *MessageStore) MarkAsRead(ctx context.Context, roomID, userID string, messageID *string) error {
	// 第一步：只更新那些 read_by 中不包含該 userID 的訊息
	filter := bson.M{
		"room_id":         roomID,
		"read_by.user_id": bson.M{"$ne": userID}, // 只選擇該用戶未讀的訊息
	}

	if messageID != nil {
		objectID, err := bson.ObjectIDFromHex(*messageID)
		if err != nil {
			return err
		}
		filter["_id"] = objectID
	}

	now := time.Now()
	readBy := MessageReadBy{
		UserID: userID,
		ReadAt: now,
	}

	// 使用 $push 添加新記錄
	// 因為 filter 已經排除了已讀的訊息，所以不會產生重複
	update := bson.M{
		"$push": bson.M{"read_by": readBy},
		"$set":  bson.M{"updated_at": now},
	}

	_, err := s.collection.UpdateMany(ctx, filter, update)
	return err
}

// MarkAsDelivered 標記消息為已送達
func (s *MessageStore) MarkAsDelivered(ctx context.Context, roomID, userID string, messageID *string) error {
	filter := bson.M{"room_id": roomID}

	if messageID != nil {
		objectID, err := bson.ObjectIDFromHex(*messageID)
		if err != nil {
			return err
		}
		filter["_id"] = objectID
	}

	// 添加已送達記錄
	deliveredTo := MessageDeliveredTo{
		UserID:      userID,
		DeliveredAt: time.Now(),
	}

	_, err := s.collection.UpdateMany(ctx, filter, bson.M{
		"$addToSet": bson.M{"delivered_to": deliveredTo},
		"$set":      bson.M{"updated_at": time.Now()},
	})
	return err
}

// GetUnreadCount 獲取未讀消息數量
func (s *MessageStore) GetUnreadCount(ctx context.Context, userID string, roomID *string) (int, error) {
	filter := bson.M{
		"read_by.user_id": bson.M{"$ne": userID},
	}

	if roomID != nil {
		filter["room_id"] = *roomID
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	return int(count), err
}

// Search 搜索消息
func (s *MessageStore) Search(ctx context.Context, roomID, query string, userID *string, messageType *string, since, until *time.Time, limit int, cursor string) ([]*Message, string, bool, int, error) {
	filter := bson.M{
		"room_id": roomID,
		"$text":   bson.M{"$search": query},
	}

	// 添加用戶過濾
	if userID != nil {
		filter["sender_id"] = *userID
	}

	// 添加消息類型過濾
	if messageType != nil {
		filter["type"] = *messageType
	}

	// 添加時間範圍過濾
	if since != nil {
		filter["created_at"] = bson.M{"$gte": *since}
	}
	if until != nil {
		if filter["created_at"] == nil {
			filter["created_at"] = bson.M{"$lte": *until}
		} else {
			filter["created_at"].(bson.M)["$lte"] = *until
		}
	}

	opts := options.Find()
	opts.SetLimit(int64(limit + 1)) // 多取一個用於判斷是否有更多
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}})

	// 如果有游標，添加游標條件
	if cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			filter["created_at"] = bson.M{"$lt": cursorTime}
		}
	}

	cursorResult, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", false, 0, err
	}
	defer cursorResult.Close(ctx)

	var messages []*Message
	for cursorResult.Next(ctx) {
		var message Message
		if decodeErr := cursorResult.Decode(&message); decodeErr != nil {
			return nil, "", false, 0, decodeErr
		}
		messages = append(messages, &message)
	}

	// 檢查是否有更多數據
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit] // 移除多取的那一個
	}

	// 生成下一個游標
	var nextCursor string
	if hasMore && len(messages) > 0 {
		nextCursor = messages[len(messages)-1].CreatedAt.Format(time.RFC3339)
	}

	// 獲取總數
	totalCount, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, "", false, 0, err
	}

	return messages, nextCursor, hasMore, int(totalCount), nil
}
