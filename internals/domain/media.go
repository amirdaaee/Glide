package domain

import (
	"context"
	"io"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MediaStatus is the processing state of a stored media file.
type MediaStatus string

const (
	// MediaStatusReceived is the initial status after the bot accepts a file.
	MediaStatusReceived MediaStatus = "received"
	// MediaStatusReady means the file is stored and available.
	MediaStatusReady MediaStatus = "ready"
	// MediaStatusFailed means ingest failed.
	MediaStatusFailed MediaStatus = "failed"
)

// MediaFileMeta is Telegram document metadata copied onto a media record.
type MediaFileMeta struct {
	FileSize int64   `bson:"FileSize"`
	FileName string  `bson:"FileName"`
	MimeType string  `bson:"MimeType"`
	FileID   int64   `bson:"FileID"`
	Duration float64 `bson:"Duration"`
}

// MediaFile is a media record owned by users and processed by the ingest pipeline.
type MediaFile struct {
	mongox.Model `bson:",inline"`
	MessageID    int           `bson:"MessageID"`
	Meta         MediaFileMeta `bson:"Meta"`
	Status       MediaStatus   `bson:"Status"`
	StorageURL   string        `bson:"StorageURL,omitempty"`
	ThumbnailURL string        `bson:"ThumbnailURL,omitempty"`
	Byse         *ByseFile     `bson:"Byse,omitempty" json:"byse,omitempty"`
}

// HasByse reports whether the file is stored on Byse.
func (m *MediaFile) HasByse() bool {
	return m != nil && m.Byse != nil && m.Byse.FileCode != ""
}

// MediaStatusReply is the Telegram message the bot will edit when this media
// reaches a terminal status. One pending reply per client per media.
type MediaStatusReply struct {
	mongox.Model `bson:",inline"`
	MediaID      bson.ObjectID `bson:"MediaID"`
	UserID       bson.ObjectID `bson:"UserID"`
	TelegramID   int64         `bson:"TelegramID"`
	AccessHash   int64         `bson:"AccessHash"`
	MessageID    int           `bson:"MessageID"`
}

// IMediaStatusReplyRepository persists pending Telegram status-reply messages.
type IMediaStatusReplyRepository interface {
	// Upsert creates or replaces the pending status reply for a media/user pair.
	Upsert(ctx context.Context, reply *MediaStatusReply) error
	// ListByMediaID returns pending status replies for a media file.
	ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*MediaStatusReply, error)
	// DeleteByMediaID removes all pending status replies for a media file.
	DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error
	// Delete removes the pending status reply for a media/user pair.
	Delete(ctx context.Context, mediaID, userID bson.ObjectID) error
}

// IMediaRepository persists media file records.
type IMediaRepository interface {
	// Create inserts a media file, assigning an ID if missing.
	Create(ctx context.Context, media *MediaFile) error
	// Get returns a media file by ID.
	Get(ctx context.Context, id bson.ObjectID) (*MediaFile, error)
	// GetByFID returns a media file by Telegram file ID.
	GetByFID(ctx context.Context, fid int64) (*MediaFile, error)
	// GetMany returns media files for the given IDs.
	GetMany(ctx context.Context, ids []bson.ObjectID) ([]*MediaFile, error)
	// Delete removes a media file by Telegram file ID.
	Delete(ctx context.Context, fid int64) error
	// SetStatus updates the media processing status.
	SetStatus(ctx context.Context, id bson.ObjectID, status MediaStatus) error
	// SetStorageURL stores the public media URL.
	SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error
	// SetThumbnailURL stores the thumbnail URL.
	SetThumbnailURL(ctx context.Context, id bson.ObjectID, url string) error
	// SetStored records the Byse file and marks media ready.
	SetStored(ctx context.Context, id bson.ObjectID, byse *ByseFile) error
}

// IMediaObjectRepository stores media object bytes (files and thumbnails).
type IMediaObjectRepository interface {
	// PutThumbnail stores a thumbnail object and returns its path.
	PutThumbnail(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
	// Put stores a media object and returns its path.
	Put(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error)
}

// ByseFile is a file stored on the Byse host.
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

// ByseListFilter selects files when listing from Byse.
type ByseListFilter struct {
	FolderID *int
	Title    string
	Created  string
	Public   *bool
	PerPage  int
	Page     int
}

// IByseMediaRepository is the Byse file storage API.
type IByseMediaRepository interface {
	// Create uploads a file to Byse and returns the new file code.
	Create(ctx context.Context, name string, body io.Reader, size int64, contentType string) (*ByseFile, error)
	// Get returns Byse file info by file code.
	Get(ctx context.Context, fileCode string) (*ByseFile, error)
	// List returns Byse files matching filter.
	List(ctx context.Context, filter ByseListFilter) ([]*ByseFile, error)
	// SetFolder moves a Byse file into folderID.
	SetFolder(ctx context.Context, fileCode string, folderID int) error
	// Clone duplicates a Byse file and returns the new file.
	Clone(ctx context.Context, fileCode string) (*ByseFile, error)
}
