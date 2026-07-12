package config

import (
	"os"
	"sync"

	"github.com/amirdaaee/Glide/internals/log"
	env "github.com/caarlos0/env/v11"
	godotenv "github.com/joho/godotenv"
	"go.uber.org/zap"
)

var configInitLock *sync.Once = &sync.Once{}
var configInstance *ConfigType

// Config returns a singleton instance of ConfigType, loading environment variables and .env file if present.
func Config() *ConfigType {
	ll := log.Named(log.CONFIG, "Config")
	configInitLock.Do(func() {
		if _, error := os.Stat(".env"); !os.IsNotExist(error) {
			ll.Info("found .env file")
			if err := godotenv.Load(); err != nil {
				ll.Panic("error loading .env file", zap.Error(err))
			}
		} else {
			ll.Warn("no .env file found")
		}
		configInstance = &ConfigType{}
		if err := env.Parse(configInstance); err != nil {
			ll.Fatal("error parsing config", zap.Error(err))
		}
		ll.Info("config parsed")
	})
	return configInstance
}
