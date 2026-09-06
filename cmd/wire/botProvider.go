package wire

import (
	"fmt"
	"strings"

	"github.com/amirdaaee/Glide/internals/bot"
	bothandler "github.com/amirdaaee/Glide/internals/bot/handler"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository/mongo"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/chenmingyong0423/go-mongox/v2"
)

func ProvideSessionConfig(cfg *config.ConfigType) *tlg.SessionConfig {
	return &tlg.SessionConfig{
		SocksProxy: cfg.TelegramConfig.SocksProxy,
		SessionDir: cfg.TelegramConfig.SessionDir,
		AppID:      cfg.TelegramConfig.AppID,
		AppHash:    cfg.TelegramConfig.AppHash,
	}
}

func ProvideMediaRepository(cfg *config.ConfigType, db *mongox.Database) domain.IMediaRepository {
	return mongo.NewMediaRepository(db, cfg.MongoConfig.MediaCollection)
}

func ProvideJobRepository(cfg *config.ConfigType, db *mongox.Database) (domain.IJobRepository, error) {
	return mongo.NewJobRepository(db, cfg.MongoConfig.JobsCollection, cfg.MongoConfig.ProcessedTasksCollection)
}

func ProvideBotClient(sessCfg *tlg.SessionConfig, cfg *config.ConfigType) (tlg.IClient, error) {
	return tlg.NewTgClient(sessCfg, cfg.BotConfig.Token, "bot")
}

func ProvideWorkerPool(sessCfg *tlg.SessionConfig, cfg *config.ConfigType) (worker.IWorkerPool, error) {
	tokens := cfg.WorkerConfig.Tokens
	if len(tokens) == 0 {
		return nil, fmt.Errorf("WORKER_TOKENS is empty")
	}
	workers := make([]*worker.Worker, 0, len(tokens))
	for i, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("WORKER_TOKENS entry %d is empty", i)
		}
		prefix := cfg.WorkerConfig.SessionPrefix
		cl, err := tlg.NewTgClient(sessCfg, token, prefix)
		if err != nil {
			return nil, fmt.Errorf("can not create worker client %d: %w", i, err)
		}
		workers = append(workers, worker.NewWorker(cl, cfg.BotConfig.ChannelID, cfg.BotConfig.ChannelAccessHash))
	}
	return worker.NewPool(workers), nil
}

func ProvideBotHandlers(
	cl tlg.IClient,
	cfg *config.ConfigType,
	wPool worker.IWorkerPool,
	media service.IMediaService,
) ([]bothandler.IHandler, error) {
	h, err := bothandler.NewMediaHandler(
		cl,
		cfg.BotConfig.ChannelID,
		cfg.BotConfig.ChannelAccessHash,
		wPool,
		media,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create media handler: %w", err)
	}
	return []bothandler.IHandler{h}, nil
}

func ProvideBot(cl tlg.IClient, wPool worker.IWorkerPool, handlers []bothandler.IHandler) (*bot.Bot, error) {
	return bot.NewBot(cl, wPool, handlers)
}
