package service

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// IMediaService is the application API for media records and ingest dispatch.
type IMediaService interface {
	// ListIDsForTelegramUser returns media IDs owned by the Telegram user.
	ListIDsForTelegramUser(ctx context.Context, telegramID int64) ([]bson.ObjectID, error)
	// Get returns a media file by ID.
	Get(ctx context.Context, id bson.ObjectID) (*domain.MediaFile, error)
	// GetByFID returns a media file by Telegram file ID.
	GetByFID(ctx context.Context, fid int64) (*domain.MediaFile, error)
	// GetOwnedByFID returns a media file owned by the Telegram user, looked up by file ID.
	GetOwnedByFID(ctx context.Context, telegramID int64, fid int64) (*domain.MediaFile, error)
	// UnlinkOwnedByFID detaches a media file from the Telegram user.
	UnlinkOwnedByFID(ctx context.Context, telegramID int64, fid int64) error
	// EnsureAttached gets or creates user and media, then attaches media to the user.
	EnsureAttached(ctx context.Context, user *domain.User, media *domain.MediaFile) (*domain.User, *domain.MediaFile, error)
	// Dispatch starts ingest-pipeline work for media.
	Dispatch(ctx context.Context, media *domain.MediaFile) error
	// SetStatus updates the media processing status.
	SetStatus(ctx context.Context, id bson.ObjectID, status domain.MediaStatus) error
	// SetStorageURL stores the public media URL.
	SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error
	// SetThumbnailURL stores the thumbnail URL.
	SetThumbnailURL(ctx context.Context, id bson.ObjectID, url string) error
	// SetStored records the Byse file and marks media ready.
	SetStored(ctx context.Context, id bson.ObjectID, byse *domain.ByseFile) error
}

// MediaService implements IMediaService.
type MediaService struct {
	media  domain.IMediaRepository
	users  IUserService
	ingest IIngestDispatcher
}

var _ IMediaService = (*MediaService)(nil)

// ListIDsForTelegramUser returns media IDs owned by the Telegram user.
func (s *MediaService) ListIDsForTelegramUser(ctx context.Context, telegramID int64) ([]bson.ObjectID, error) {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	return user.MediaList, nil
}

// Get returns a media file by ID.
func (s *MediaService) Get(ctx context.Context, id bson.ObjectID) (*domain.MediaFile, error) {
	media, err := s.media.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("can not get media file: %w", err)
	}
	return media, nil
}

// GetByFID returns a media file by Telegram file ID.
func (s *MediaService) GetByFID(ctx context.Context, fid int64) (*domain.MediaFile, error) {
	media, err := s.media.GetByFID(ctx, fid)
	if err != nil {
		return nil, fmt.Errorf("can not get media file by fid: %w", err)
	}
	return media, nil
}

// GetOwnedByFID returns a media file owned by the Telegram user, looked up by file ID.
func (s *MediaService) GetOwnedByFID(ctx context.Context, telegramID int64, fid int64) (*domain.MediaFile, error) {
	_, media, err := s.ownedMedia(ctx, telegramID, fid)
	return media, err
}

// UnlinkOwnedByFID detaches a media file from the Telegram user.
func (s *MediaService) UnlinkOwnedByFID(ctx context.Context, telegramID int64, fid int64) error {
	user, media, err := s.ownedMedia(ctx, telegramID, fid)
	if err != nil {
		return err
	}
	if err := s.users.DetachMedia(ctx, user.ID, media.ID); err != nil {
		return err
	}
	return nil
}

// EnsureAttached gets or creates user and media, then attaches media to the user.
func (s *MediaService) EnsureAttached(ctx context.Context, user *domain.User, media *domain.MediaFile) (*domain.User, *domain.MediaFile, error) {
	usr, err := s.users.GetOrCreate(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	media, err = s.getOrCreateByFID(ctx, media)
	if err != nil {
		return nil, nil, err
	}
	if err := s.users.AttachMedia(ctx, usr.ID, media.ID); err != nil {
		return nil, nil, err
	}
	return usr, media, nil
}

// Dispatch starts ingest-pipeline work for media.
func (s *MediaService) Dispatch(ctx context.Context, media *domain.MediaFile) error {
	if err := s.ingest.Dispatch(ctx, media); err != nil {
		return fmt.Errorf("can not dispatch ingest: %w", err)
	}
	return nil
}

// getOrCreateByFID returns an existing media file by Telegram file ID, or creates media.
func (s *MediaService) getOrCreateByFID(ctx context.Context, media *domain.MediaFile) (*domain.MediaFile, error) {
	return getOrCreate(
		func() (*domain.MediaFile, error) {
			existing, err := s.media.GetByFID(ctx, media.Meta.FileID)
			if err != nil {
				return nil, fmt.Errorf("can not get media file by fid: %w", err)
			}
			return existing, nil
		},
		func(m *domain.MediaFile) error {
			if err := s.media.Create(ctx, m); err != nil {
				return fmt.Errorf("can not create media file: %w", err)
			}
			return nil
		},
		media,
	)
}

// SetStatus updates the media processing status.
func (s *MediaService) SetStatus(ctx context.Context, id bson.ObjectID, status domain.MediaStatus) error {
	if !validMediaStatus(status) {
		return fmt.Errorf("media status %s: %w", status, domain.ErrInvalidTransition)
	}
	if err := s.media.SetStatus(ctx, id, status); err != nil {
		return fmt.Errorf("can not set media status: %w", err)
	}
	return nil
}

// SetStorageURL stores the public media URL.
func (s *MediaService) SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error {
	if err := s.media.SetStorageURL(ctx, id, url); err != nil {
		return fmt.Errorf("can not set media storage url: %w", err)
	}
	return nil
}

// SetThumbnailURL stores the thumbnail URL.
func (s *MediaService) SetThumbnailURL(ctx context.Context, id bson.ObjectID, url string) error {
	if err := s.media.SetThumbnailURL(ctx, id, url); err != nil {
		return fmt.Errorf("can not set media thumbnail url: %w", err)
	}
	return nil
}

// SetStored records the Byse file and marks media ready.
func (s *MediaService) SetStored(ctx context.Context, id bson.ObjectID, byse *domain.ByseFile) error {
	if err := s.media.SetStored(ctx, id, byse); err != nil {
		return fmt.Errorf("can not set stored byse file: %w", err)
	}
	return nil
}

// ownedMedia returns the user and media if the Telegram user owns the file.
func (s *MediaService) ownedMedia(ctx context.Context, telegramID int64, fid int64) (*domain.User, *domain.MediaFile, error) {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, nil, err
	}
	media, err := s.media.GetByFID(ctx, fid)
	if err != nil {
		return nil, nil, fmt.Errorf("can not get media: %w", err)
	}
	if !userOwnsMedia(user, media.ID) {
		return nil, nil, domain.ErrNotFound
	}
	return user, media, nil
}

// validMediaStatus reports whether status is a known MediaStatus value.
func validMediaStatus(status domain.MediaStatus) bool {
	switch status {
	case domain.MediaStatusReceived, domain.MediaStatusReady, domain.MediaStatusFailed:
		return true
	default:
		return false
	}
}

// userOwnsMedia reports whether mediaID is in the user's media list.
func userOwnsMedia(user *domain.User, mediaID bson.ObjectID) bool {
	for _, id := range user.MediaList {
		if id == mediaID {
			return true
		}
	}
	return false
}

// NewMediaService returns an IMediaService.
func NewMediaService(media domain.IMediaRepository, users IUserService, ingest IIngestDispatcher) IMediaService {
	return &MediaService{media: media, users: users, ingest: ingest}
}
