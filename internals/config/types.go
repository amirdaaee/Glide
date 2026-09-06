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
	URI                      string        `env:"URI,required"`
	DB                       string        `env:"DB,required"`
	UsersCollection          string        `env:"USERS_COLLECTION" envDefault:"users"`
	MediaCollection          string        `env:"MEDIA_COLLECTION" envDefault:"media"`
	JobsCollection           string        `env:"JOBS_COLLECTION" envDefault:"jobs"`
	ProcessedTasksCollection string        `env:"PROCESSED_TASKS_COLLECTION" envDefault:"processed_tasks"`
	PingTimeout              time.Duration `env:"PING_TIMEOUT" envDefault:"5s"`
}

type NatsConfigType struct {
	URL        string        `env:"URL" envDefault:"nats://127.0.0.1:4222"`
	Stream     string        `env:"STREAM" envDefault:"GLIDE_MEDIA"`
	AckWait    time.Duration `env:"ACK_WAIT" envDefault:"10m"`
	MaxDeliver int           `env:"MAX_DELIVER" envDefault:"5"`
}

type MinioConfigType struct {
	Endpoint        string `env:"ENDPOINT" envDefault:"127.0.0.1:9000"`
	AccessKeyID     string `env:"ACCESS_KEY" envDefault:"minioadmin"`
	SecretAccessKey string `env:"SECRET_KEY" envDefault:"minioadmin"`
	Bucket          string `env:"BUCKET" envDefault:"glide"`
	UseSSL          bool   `env:"USE_SSL" envDefault:"false"`
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
	Tokens        []string `env:"TOKENS,required"`
	SessionPrefix string   `env:"SESSION_PREFIX" envDefault:"worker"`
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
	NatsConfig     NatsConfigType     `envPrefix:"NATS_"`
	MinioConfig    MinioConfigType    `envPrefix:"MINIO_"`
}
