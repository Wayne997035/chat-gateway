package message

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MessageStore message 存儲實作.
type MessageStore struct {
	collection *mongo.Collection
}

// NewMessageStore 創建新的 message 存儲.
func NewMessageStore(db *mongo.Database) *MessageStore {
	return &MessageStore{
		collection: db.Collection("messages"),
	}
}

// Create 創建消息.
func (s *MessageStore) Create(ctx context.Context, message *Message) error {
	message.CreatedAt = time.Now()
	message.UpdatedAt = time.Now()
	
	_, err := s.collection.InsertOne(ctx, message)
	return err
}

// GetByID 根據 ID 獲取消息.
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

// List 列出消息.
func (s *MessageStore) List(ctx context.Context, filter map[string]interface{}, limit, offset int64) ([]*Message, error) {
	opts := options.Find()
	opts.SetLimit(limit)
	opts.SetSkip(offset)
	opts.SetSort(bson.D{{"created_at", -1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []*Message
	for cursor.Next(ctx) {
		var message Message
		if err := cursor.Decode(&message); err != nil {
			return nil, err
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

// Update 更新消息.
func (s *MessageStore) Update(ctx context.Context, id string, update map[string]interface{}) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	update["updated_at"] = time.Now()
	_, err = s.collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	return err
}

// Delete 刪除消息.
func (s *MessageStore) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	return err
}
