package message

import (
	"context"
	"time"
)

// MessageRepository message 倉儲接口.
type MessageRepository interface {
	Create(ctx context.Context, message *Message) error
	GetByID(ctx context.Context, id string) (*Message, error)
	List(ctx context.Context, filter map[string]interface{}, limit, offset int64) ([]*Message, error)
	Update(ctx context.Context, id string, update map[string]interface{}) error
	Delete(ctx context.Context, id string) error
}

// Message message 數據模型.
type Message struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	Content   string    `bson:"content" json:"content"`
	SenderID  string    `bson:"sender_id" json:"sender_id"`
	ChannelID string    `bson:"channel_id" json:"channel_id"`
	Type      string    `bson:"type" json:"type"`     // text, image, file, etc.
	Status    string    `bson:"status" json:"status"` // sent, delivered, read
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
