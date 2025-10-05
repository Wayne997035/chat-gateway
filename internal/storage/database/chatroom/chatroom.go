package chatroom

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ChatRoomRepository 聊天室倉儲接口
type ChatRoomRepository interface {
	Create(ctx context.Context, room *ChatRoom) error
	GetByID(ctx context.Context, id string) (*ChatRoom, error)
	Update(ctx context.Context, id string, update map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string, limit int, cursor string) ([]*ChatRoom, string, bool, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	AddMember(ctx context.Context, member *RoomMember) error
	RemoveMember(ctx context.Context, roomID, userID string) error
	GetMembers(ctx context.Context, roomID string) ([]RoomMember, error)
	GetMemberCount(ctx context.Context, roomID string) (int, error)
}

// ChatRoom 聊天室數據模型
type ChatRoom struct {
	_ID             interface{}            `bson:"_id" form:"_id"`
	ID              string                 `json:"id,omitempty" bson:"id" form:"id"`
	Name            string                 `bson:"name" json:"name"`
	AvatarURL       string                 `bson:"avatar_url" json:"avatar_url"`
	Type            string                 `bson:"type" json:"type"`
	OwnerID         string                 `bson:"owner_id" json:"owner_id"`
	Settings        RoomSettings           `bson:"settings" json:"settings"`
	CreatedAt       time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time              `bson:"updated_at" json:"updated_at"`
	LastMessageAt   time.Time              `bson:"last_message_at" json:"last_message_at"`
	LastMessage     string                 `bson:"last_message" json:"last_message"`
	LastMessageTime time.Time              `bson:"last_message_time" json:"last_message_time"`
	Members         []RoomMember           `bson:"members,omitempty" json:"members,omitempty"`
	Metadata        map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// NewChatRoom 創建新的 ChatRoom 實例
func NewChatRoom() ChatRoom {
	_id := bson.NewObjectID()
	now := time.Now().UTC()
	return ChatRoom{_ID: _id, ID: _id.Hex(), CreatedAt: now, UpdatedAt: now, LastMessageAt: now}
}

// RoomMember 聊天室成員數據模型
type RoomMember struct {
	UserID      string    `bson:"user_id" json:"user_id"`
	Username    string    `bson:"username" json:"username"`
	DisplayName string    `bson:"display_name" json:"display_name"`
	AvatarURL   string    `bson:"avatar_url" json:"avatar_url"`
	Role        string    `bson:"role" json:"role"`
	Status      string    `bson:"status" json:"status"`
	JoinedAt    time.Time `bson:"joined_at" json:"joined_at"`
	LastSeen    time.Time `bson:"last_seen" json:"last_seen"`
	LastReadAt  time.Time `bson:"last_read_at" json:"last_read_at"`
}

// RoomSettings 聊天室設置數據模型
type RoomSettings struct {
	AllowInvite         bool   `bson:"allow_invite" json:"allow_invite"`
	AllowEditMessages   bool   `bson:"allow_edit_messages" json:"allow_edit_messages"`
	AllowDeleteMessages bool   `bson:"allow_delete_messages" json:"allow_delete_messages"`
	AllowPinMessages    bool   `bson:"allow_pin_messages" json:"allow_pin_messages"`
	MaxMembers          int    `bson:"max_members" json:"max_members"`
	WelcomeMessage      string `bson:"welcome_message" json:"welcome_message"`
}

// ChatRoomStore 聊天室存儲實作
type ChatRoomStore struct {
	collection *mongo.Collection
}

// NewChatRoomStore 創建新的聊天室存儲
func NewChatRoomStore(db *mongo.Database) *ChatRoomStore {
	return &ChatRoomStore{
		collection: db.Collection("chat_rooms"),
	}
}

// Create 創建聊天室
func (s *ChatRoomStore) Create(ctx context.Context, room *ChatRoom) error {
	_id := bson.NewObjectID()
	room._ID = _id
	room.ID = _id.Hex()
	room.CreatedAt = time.Now()
	room.UpdatedAt = time.Now()
	room.LastMessageAt = time.Now()

	_, err := s.collection.InsertOne(ctx, room)
	return err
}

// GetByID 根據 ID 獲取聊天室
func (s *ChatRoomStore) GetByID(ctx context.Context, id string) (*ChatRoom, error) {
	objectID, err := parseObjectID(id)
	if err != nil {
		return nil, err
	}

	var room ChatRoom
	err = s.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// Update 更新聊天室
func (s *ChatRoomStore) Update(ctx context.Context, id string, update map[string]interface{}) error {
	update["updated_at"] = time.Now()
	_, err := s.collection.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": update})
	return err
}

// Delete 刪除聊天室
func (s *ChatRoomStore) Delete(ctx context.Context, id string) error {
	objectID, err := parseObjectID(id)
	if err != nil {
		return err
	}
	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	return err
}

// parseObjectID 是轉換字符串 ID 為 ObjectID 的輔助函數
func parseObjectID(id string) (bson.ObjectID, error) {
	return bson.ObjectIDFromHex(id)
}

// ListUserRooms 列出用戶的聊天室.
func (s *ChatRoomStore) ListUserRooms(
	ctx context.Context, userID string, limit int, cursor string,
) (
	rooms []*ChatRoom, nextCursor string, hasMore bool, err error,
) {
	filter := bson.M{
		"members.user_id": userID,
	}

	opts := options.Find()
	opts.SetLimit(int64(limit + 1)) // 多取一個用於判斷是否有更多
	opts.SetSort(bson.D{{Key: "last_message_at", Value: -1}})

	// 如果有游標，添加游標條件
	if cursor != "" {
		cursorTime, parseErr := time.Parse(time.RFC3339, cursor)
		if parseErr == nil {
			filter["last_message_at"] = bson.M{"$lt": cursorTime}
		}
	}

	cursorResult, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", false, err
	}
	defer cursorResult.Close(ctx)

	rooms = []*ChatRoom{}
	for cursorResult.Next(ctx) {
		var room ChatRoom
		if err := cursorResult.Decode(&room); err != nil {
			return nil, "", false, err
		}
		rooms = append(rooms, &room)
	}

	// 檢查是否有更多數據
	hasMore = len(rooms) > limit
	if hasMore {
		rooms = rooms[:limit] // 移除多取的那一個
	}

	// 生成下一個游標
	if hasMore && len(rooms) > 0 {
		nextCursor = rooms[len(rooms)-1].LastMessageAt.Format(time.RFC3339)
	}

	return rooms, nextCursor, hasMore, nil
}

// IsMember 檢查用戶是否是聊天室成員
func (s *ChatRoomStore) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	count, err := s.collection.CountDocuments(ctx, bson.M{
		"id":              roomID,
		"members.user_id": userID,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// AddMember 添加成員
func (s *ChatRoomStore) AddMember(ctx context.Context, roomID string, member *RoomMember) error {
	member.JoinedAt = time.Now()
	member.LastSeen = time.Now()
	member.LastReadAt = time.Now()

	result, err := s.collection.UpdateOne(ctx, bson.M{"id": roomID}, bson.M{
		"$push": bson.M{"members": member},
		"$set":  bson.M{"updated_at": time.Now()},
	})

	if err != nil {
		return fmt.Errorf("update failed: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("room not found: %s", roomID)
	}

	if result.ModifiedCount == 0 {
		return fmt.Errorf("no changes made to room: %s", roomID)
	}

	return nil
}

// RemoveMember 移除成員
func (s *ChatRoomStore) RemoveMember(ctx context.Context, roomID, userID string) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"id": roomID}, bson.M{
		"$pull": bson.M{"members": bson.M{"user_id": userID}},
		"$set":  bson.M{"updated_at": time.Now()},
	})
	return err
}

// GetMembers 獲取聊天室成員
func (s *ChatRoomStore) GetMembers(ctx context.Context, roomID string) ([]RoomMember, error) {
	objectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, err
	}

	var room ChatRoom
	err = s.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&room)
	if err != nil {
		return nil, err
	}

	return room.Members, nil
}

// GetMemberCount 獲取聊天室成員數量
func (s *ChatRoomStore) GetMemberCount(ctx context.Context, roomID string) (int, error) {
	objectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return 0, err
	}

	var room ChatRoom
	err = s.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&room)
	if err != nil {
		return 0, err
	}

	return len(room.Members), nil
}
