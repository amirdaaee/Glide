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
	mustProvide(c, "mongoDB", ProvideMongoDB)
	mustProvide(c, "api", ProvideAPIServer)
	mustProvide(c, "userRepository", ProvideUserRepository)
	mustProvide(c, "apiHandlers", ProvideApiHandlers)
	mustProvide(c, "config", ProvideConfig)
	return c
}
