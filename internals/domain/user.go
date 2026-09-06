package domain

import (
	"context"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	mongox.Model `bson:",inline"`
	TelegramID   int64           `bson:"TelegramID"`
	Username     string          `bson:"Username"`
	FirstName    string          `bson:"FirstName"`
	LastName     string          `bson:"LastName"`
	LanguageCode string          `bson:"LanguageCode"`
	MediaList    []bson.ObjectID `bson:"MediaList"`
}

type IUserRepository interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	Create(ctx context.Context, user *User) error
	Save(ctx context.Context, user *User) error
	AddMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
	DeleteMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
}
