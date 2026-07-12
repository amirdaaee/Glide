package domain

import (
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
