package wire

import (
	"github.com/amirdaaee/Glide/internals/log"
	"go.uber.org/dig"
	"go.uber.org/zap"
)

func mustProvide(c *dig.Container, name string, ctor any) {
	if err := c.Provide(ctor); err != nil {
		log.GetLogger(log.CMD).With(zap.Error(err), zap.String("provider", name)).Fatal("wire: register provider")
	}
}
func GetProvider() *dig.Container {
	c := dig.New()
	mustProvide(c, "config", ProvideConfig)
	mustProvide(c, "mongoDB", ProvideMongoDB)
	mustProvide(c, "userRepository", ProvideUserRepository)
	mustProvide(c, "mediaRepository", ProvideMediaRepository)
	mustProvide(c, "apiHandlers", ProvideApiHandlers)
	mustProvide(c, "api", ProvideAPIServer)
	mustProvide(c, "sessionConfig", ProvideSessionConfig)
	mustProvide(c, "botClient", ProvideBotClient)
	mustProvide(c, "workerPool", ProvideWorkerPool)
	mustProvide(c, "botHandlers", ProvideBotHandlers)
	mustProvide(c, "bot", ProvideBot)
	return c
}
