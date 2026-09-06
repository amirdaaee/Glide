package domain

import (
	"context"
	"io"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
	ThumbnailURL string        `bson:"ThumbnailURL,omitempty"`
	Byse         *ByseFile     `bson:"Byse,omitempty" json:"byse,omitempty"`
}

func (m *MediaFile) HasByse() bool {
	return m != nil && m.Byse != nil && m.Byse.FileCode != ""
}

type IMediaRepository interface {
	Create(ctx context.Context, media *MediaFile) error
	Get(ctx context.Context, id bson.ObjectID) (*MediaFile, error)
	GetByFID(ctx context.Context, fid int64) (*MediaFile, error)
	GetMany(ctx context.Context, ids []bson.ObjectID) ([]*MediaFile, error)
	Delete(ctx context.Context, fid int64) error
	SetStatus(ctx context.Context, id bson.ObjectID, status MediaStatus) error
	SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error
	SetThumbnailURL(ctx context.Context, id bson.ObjectID, url string) error
	SetStored(ctx context.Context, id bson.ObjectID, byse *ByseFile) error
}

type IMediaObjectRepository interface {
	PutThumbnail(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
	Put(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
}

type ByseFile struct {
	FileCode  string `bson:"FileCode,omitempty" json:"file_code,omitempty"`
	Name      string `bson:"Name,omitempty" json:"name,omitempty"`
	CanPlay   bool   `bson:"CanPlay,omitempty" json:"can_play,omitempty"`
	Duration  int    `bson:"Duration,omitempty" json:"duration,omitempty"`
	Views     int    `bson:"Views,omitempty" json:"views,omitempty"`
	Uploaded  string `bson:"Uploaded,omitempty" json:"uploaded,omitempty"`
	FolderID  int    `bson:"FolderID,omitempty" json:"folder_id,omitempty"`
	Public    bool   `bson:"Public,omitempty" json:"public,omitempty"`
	Thumbnail string `bson:"Thumbnail,omitempty" json:"thumbnail,omitempty"`
	Link      string `bson:"Link,omitempty" json:"link,omitempty"`
}

type ByseListFilter struct {
	FolderID *int
	Title    string
	Created  string
	Public   *bool
	PerPage  int
	Page     int
}

type IByseMediaRepository interface {
	Create(ctx context.Context, name string, body io.Reader, size int64, contentType string) (*ByseFile, error)
	Get(ctx context.Context, fileCode string) (*ByseFile, error)
	List(ctx context.Context, filter ByseListFilter) ([]*ByseFile, error)
	SetFolder(ctx context.Context, fileCode string, folderID int) error
	Clone(ctx context.Context, fileCode string) (*ByseFile, error)
}
