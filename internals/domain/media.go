package domain

import "github.com/chenmingyong0423/go-mongox/v2"

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
}
