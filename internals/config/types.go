package config

import "time"

type ApiConfigType struct {
	Listen string `env:"LISTEN" envDefault:":8080"`
}

type MetricsConfigType struct {
	Enabled bool   `env:"METRICS_ENABLED" envDefault:"true"`
	Address string `env:"METRICS_ADDRESS" envDefault:":9090"`
}

type AuthConfigType struct {
	ClientID      string `env:"CLIENT_ID,required"`
	ClientSecret  string `env:"CLIENT_SECRET,required"`
	RedirectURL   string `env:"REDIRECT_URL,required"`
	SecureCookies bool   `env:"SECURE_COOKIES" envDefault:"true"`
}

type MongoConfigType struct {
	URI             string        `env:"URI,required"`
	DB              string        `env:"DB,required"`
	UsersCollection string        `env:"USERS_COLLECTION" envDefault:"users"`
	MediaCollection string        `env:"MEDIA_COLLECTION" envDefault:"media"`
	PingTimeout     time.Duration `env:"PING_TIMEOUT" envDefault:"5s"`
}

type BotConfigType struct {
	Token             string `env:"TOKEN,required"`
	ChannelID         int64  `env:"CHANNEL_ID,required"`
	ChannelAccessHash int64  `env:"CHANNEL_ACCESS_HASH"`
}

type TelegramConfigType struct {
	AppID      int    `env:"APP_ID,required"`
	AppHash    string `env:"APP_HASH,required"`
	SocksProxy string `env:"SOCKS_PROXY"`
	SessionDir string `env:"SESSION_DIR" envDefault:"sessions"`
}

type WorkerConfigType struct {
	Tokens []string `env:"TOKENS,required"`
}

// ConfigType holds all configuration values loaded from environment variables, including API settings, MongoDB connection details, debug options, and logging preferences.
type ConfigType struct {
	ApiConfig      ApiConfigType      `envPrefix:"API_"`
	MetricsConfig  MetricsConfigType  `envPrefix:"METRICS_"`
	AuthConfig     AuthConfigType     `envPrefix:"AUTH_"`
	MongoConfig    MongoConfigType    `envPrefix:"MONGO_"`
	BotConfig      BotConfigType      `envPrefix:"BOT_"`
	TelegramConfig TelegramConfigType `envPrefix:"TELEGRAM_"`
	WorkerConfig   WorkerConfigType   `envPrefix:"WORKER_"`
}
