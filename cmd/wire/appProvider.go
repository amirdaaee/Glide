package wire

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/api"
	"github.com/amirdaaee/Glide/internals/api/handler"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository/mongo"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/chenmingyong0423/go-mongox/v2"
)

func ProvideAPIServer(cfg *config.ConfigType, h []handler.IApiHandler) *api.ApiServer {
	return api.NewApiServer(&cfg.ApiConfig, h)
}

func ProvideApiHandlers(
	cfg *config.ConfigType,
	users service.IUserService,
	media service.IMediaService,
) ([]handler.IApiHandler, error) {
	var h []handler.IApiHandler
	hAuth, err := handler.NewAuthHandler(cfg.AuthConfig, users)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth handler: %w", err)
	}
	h = append(h, hAuth)

	hMedia, err := handler.NewMediaHandler(cfg.AuthConfig, media)
	if err != nil {
		return nil, fmt.Errorf("failed to create media handler: %w", err)
	}
	h = append(h, hMedia)
	return h, nil
}

func ProvideConfig() *config.ConfigType {
	return config.Config()
}

func ProvideUserRepository(cfg *config.ConfigType, db *mongox.Database) domain.IUserRepository {
	return mongo.NewUserRepository(db, cfg.MongoConfig.UsersCollection)
}

func ProvideUserService(users domain.IUserRepository) service.IUserService {
	return service.NewUserService(users)
}

func ProvideMediaService(media domain.IMediaRepository, users service.IUserService) service.IMediaService {
	return service.NewMediaService(media, users)
}

func ProvideJobService(jobs domain.IJobRepository) service.IJobService {
	return service.NewJobService(jobs)
}
