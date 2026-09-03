package wire

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/api"
	"github.com/amirdaaee/Glide/internals/api/handler"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/amirdaaee/Glide/internals/repository/mongo"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/chenmingyong0423/go-mongox/v2"
)

func ProvideAPIServer(cfg *config.ConfigType, h []handler.IApiHandler) *api.ApiServer {
	return api.NewApiServer(&cfg.ApiConfig, h)
}

func ProvideApiHandlers(
	cfg *config.ConfigType,
	userRepo repository.IUserRepository,
	mediaRepo repository.IMediaRepository,
	wPool worker.IWorkerPool,
) ([]handler.IApiHandler, error) {
	var h []handler.IApiHandler
	hAuth, err := handler.NewAuthHandler(cfg.AuthConfig, userRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth handler: %w", err)
	}
	h = append(h, hAuth)

	hMedia, err := handler.NewMediaHandler(cfg.AuthConfig, mediaRepo, userRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create media handler: %w", err)
	}
	h = append(h, hMedia)
	return h, nil
}

func ProvideConfig() *config.ConfigType {
	return config.Config()
}
func ProvideUserRepository(cfg *config.ConfigType, db *mongox.Database) repository.IUserRepository {
	return mongo.NewUserRepository(db, cfg.MongoConfig.UsersCollection)
}
