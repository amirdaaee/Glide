package service

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// IUserService is the application API for Telegram users.
type IUserService interface {
	// GetByTelegramID returns the user for a Telegram account ID.
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	// UpsertByTelegramID creates the user if missing, then updates profile fields.
	UpsertByTelegramID(ctx context.Context, user *domain.User) error
	// GetOrCreate returns an existing user by Telegram ID, or creates user.
	GetOrCreate(ctx context.Context, user *domain.User) (*domain.User, error)
	// AttachMedia adds mediaID to the user's media list.
	AttachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
	// DetachMedia removes mediaID from the user's media list.
	DetachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
}

// UserService implements IUserService.
type UserService struct {
	users domain.IUserRepository
}

var _ IUserService = (*UserService)(nil)

// GetByTelegramID returns the user for a Telegram account ID.
func (s *UserService) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("can not get user by telegram id: %w", err)
	}
	return user, nil
}

// UpsertByTelegramID creates the user if missing, then updates profile fields.
func (s *UserService) UpsertByTelegramID(ctx context.Context, user *domain.User) error {
	got, err := s.GetOrCreate(ctx, user)
	if err != nil {
		return err
	}
	if got == user {
		return nil
	}
	got.Username = user.Username
	got.FirstName = user.FirstName
	got.LastName = user.LastName
	if user.LanguageCode != "" {
		got.LanguageCode = user.LanguageCode
	}
	if err := s.users.Save(ctx, got); err != nil {
		return fmt.Errorf("can not save user: %w", err)
	}
	return nil
}

// GetOrCreate returns an existing user by Telegram ID, or creates user.
func (s *UserService) GetOrCreate(ctx context.Context, user *domain.User) (*domain.User, error) {
	return getOrCreate(
		func() (*domain.User, error) {
			existing, err := s.users.GetByTelegramID(ctx, user.TelegramID)
			if err != nil {
				return nil, fmt.Errorf("can not get user by telegram id: %w", err)
			}
			return existing, nil
		},
		func(u *domain.User) error {
			if u.MediaList == nil {
				u.MediaList = []bson.ObjectID{}
			}
			if err := s.users.Create(ctx, u); err != nil {
				return fmt.Errorf("can not create user: %w", err)
			}
			return nil
		},
		user,
	)
}

// AttachMedia adds mediaID to the user's media list.
func (s *UserService) AttachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error {
	if err := s.users.AddMedia(ctx, userID, mediaID); err != nil {
		return fmt.Errorf("can not add media to user: %w", err)
	}
	return nil
}

// DetachMedia removes mediaID from the user's media list.
func (s *UserService) DetachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error {
	if err := s.users.DeleteMedia(ctx, userID, mediaID); err != nil {
		return fmt.Errorf("can not delete media from user: %w", err)
	}
	return nil
}

// NewUserService returns an IUserService.
func NewUserService(users domain.IUserRepository) IUserService {
	return &UserService{users: users}
}
