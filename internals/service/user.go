package service

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IUserService interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	UpsertByTelegramID(ctx context.Context, user *domain.User) error
	GetOrCreate(ctx context.Context, user *domain.User) (*domain.User, error)
	AttachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
	DetachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
}

type UserService struct {
	users domain.IUserRepository
}

var _ IUserService = (*UserService)(nil)

func (s *UserService) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("can not get user by telegram id: %w", err)
	}
	return user, nil
}

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

func (s *UserService) AttachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error {
	if err := s.users.AddMedia(ctx, userID, mediaID); err != nil {
		return fmt.Errorf("can not add media to user: %w", err)
	}
	return nil
}

func (s *UserService) DetachMedia(ctx context.Context, userID, mediaID bson.ObjectID) error {
	if err := s.users.DeleteMedia(ctx, userID, mediaID); err != nil {
		return fmt.Errorf("can not delete media from user: %w", err)
	}
	return nil
}

func NewUserService(users domain.IUserRepository) IUserService {
	return &UserService{users: users}
}
