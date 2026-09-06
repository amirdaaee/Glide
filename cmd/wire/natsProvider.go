package wire

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/orchestrator"
	"github.com/amirdaaee/Glide/internals/pipeline"
	pipelinenats "github.com/amirdaaee/Glide/internals/pipeline/nats"
	"github.com/amirdaaee/Glide/internals/service"
)

func ProvideNatsClient(cfg *config.ConfigType) (*pipelinenats.Client, error) {
	cl, err := pipelinenats.NewClient(&cfg.NatsConfig)
	if err != nil {
		return nil, fmt.Errorf("can not create nats client: %w", err)
	}
	return cl, nil
}

func ProvidePublisher(cl *pipelinenats.Client) pipeline.IPublisher {
	return cl
}

func ProvideSubscriber(cl *pipelinenats.Client) pipeline.ISubscriber {
	return cl
}

func ProvideOrchestrator(
	sub pipeline.ISubscriber,
	pub pipeline.IPublisher,
	media service.IMediaService,
	jobs service.IJobService,
	cfg *config.ConfigType,
) (*orchestrator.Orchestrator, error) {
	return orchestrator.New(sub, pub, media, jobs, cfg.BotConfig.ChannelID)
}
