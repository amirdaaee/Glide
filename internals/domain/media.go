package domain

import "github.com/chenmingyong0423/go-mongox/v2"

type MediaStatus string

const (
	MediaStatusReceived MediaStatus = "received"
	MediaStatusReady    MediaStatus = "ready"
	MediaStatusFailed   MediaStatus = "failed"
)

type MediaFileMeta struct {
	FileSize int64   `bson:"FileSize"`
	FileName string  `bson:"FileName"`
	MimeType string  `bson:"MimeType"`
	FileID   int64   `bson:"FileID"`
	Duration float64 `bson:"Duration"`
}

type MediaFile struct {
	mongox.Model `bson:",inline"`
	MessageID    int           `bson:"MessageID"`
	Meta         MediaFileMeta `bson:"Meta"`
	Status       MediaStatus   `bson:"Status"`
	StorageURL   string        `bson:"StorageURL,omitempty"`
}
