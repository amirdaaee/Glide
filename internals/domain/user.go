package domain

import (
	"context"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// User is a Telegram user and the media files they own.
type User struct {
	mongox.Model `bson:",inline"`
	TelegramID   int64           `bson:"TelegramID"`
	Username     string          `bson:"Username"`
	FirstName    string          `bson:"FirstName"`
	LastName     string          `bson:"LastName"`
	LanguageCode string          `bson:"LanguageCode"`
	MediaList    []bson.ObjectID `bson:"MediaList"`
}

// IUserRepository persists users and their media attachments.
type IUserRepository interface {
	// GetByTelegramID returns the user for a Telegram account ID.
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	// Create inserts a user.
	Create(ctx context.Context, user *User) error
	// Save updates user fields by ID.
	Save(ctx context.Context, user *User) error
	// AddMedia attaches mediaID to the user's media list.
	AddMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
	// DeleteMedia detaches mediaID from the user's media list.
	DeleteMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
}
