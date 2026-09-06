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
}

type IMediaRepository interface {
	Create(ctx context.Context, media *MediaFile) error
	GetByFID(ctx context.Context, fid int64) (*MediaFile, error)
	GetMany(ctx context.Context, ids []bson.ObjectID) ([]*MediaFile, error)
	Delete(ctx context.Context, fid int64) error
	SetStatus(ctx context.Context, id bson.ObjectID, status MediaStatus) error
	SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error
	SetThumbnailURL(ctx context.Context, id bson.ObjectID, url string) error
}

type IMediaObjectRepository interface {
	PutThumbnail(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
	Put(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
}

type ByseFile struct {
	FileCode  string
	Name      string
	CanPlay   bool
	Duration  int
	Views     int
	Uploaded  string
	FolderID  int
	Public    bool
	Thumbnail string
	Link      string
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
