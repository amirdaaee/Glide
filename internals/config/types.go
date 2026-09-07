package config

import "time"

// ApiConfigType is HTTP listen configuration.
type ApiConfigType struct {
	Listen string `env:"LISTEN" envDefault:":8080"`
}

// AuthConfigType is Telegram OIDC client configuration.
type AuthConfigType struct {
	ClientID      string `env:"CLIENT_ID,required"`
	ClientSecret  string `env:"CLIENT_SECRET,required"`
	RedirectURL   string `env:"REDIRECT_URL,required"`
	SecureCookies bool   `env:"SECURE_COOKIES" envDefault:"true"`
}

// MongoConfigType is MongoDB connection and collection names.
type MongoConfigType struct {
	URI                      string        `env:"URI,required"`
	DB                       string        `env:"DB,required"`
	UsersCollection          string        `env:"USERS_COLLECTION" envDefault:"users"`
	MediaCollection          string        `env:"MEDIA_COLLECTION" envDefault:"media"`
	JobsCollection           string        `env:"JOBS_COLLECTION" envDefault:"jobs"`
	ProcessedTasksCollection string        `env:"PROCESSED_TASKS_COLLECTION" envDefault:"processed_tasks"`
	StatusRepliesCollection  string        `env:"STATUS_REPLIES_COLLECTION" envDefault:"status_replies"`
	PingTimeout              time.Duration `env:"PING_TIMEOUT" envDefault:"5s"`
}

// NatsConfigType is NATS JetStream connection settings.
type NatsConfigType struct {
	URL        string        `env:"URL" envDefault:"nats://127.0.0.1:4222"`
	Stream     string        `env:"STREAM" envDefault:"GLIDE_MEDIA"`
	AckWait    time.Duration `env:"ACK_WAIT" envDefault:"30m"`
	MaxDeliver int           `env:"MAX_DELIVER" envDefault:"5"`
}

// MinioConfigType is MinIO object-store credentials.
type MinioConfigType struct {
	Endpoint        string `env:"ENDPOINT" envDefault:"127.0.0.1:9000"`
	AccessKeyID     string `env:"ACCESS_KEY" envDefault:"minioadmin"`
	SecretAccessKey string `env:"SECRET_KEY" envDefault:"minioadmin"`
	Bucket          string `env:"BUCKET" envDefault:"glide"`
	UseSSL          bool   `env:"USE_SSL" envDefault:"false"`
}

// BotConfigType is the Telegram bot token and storage channel.
type BotConfigType struct {
	Token     string `env:"TOKEN,required"`
	ChannelID int64  `env:"CHANNEL_ID,required"`
}

// TelegramConfigType is MTProto app credentials and session storage.
type TelegramConfigType struct {
	AppID      int    `env:"APP_ID,required"`
	AppHash    string `env:"APP_HASH,required"`
	SocksProxy string `env:"SOCKS_PROXY"`
	SessionDir string `env:"SESSION_DIR" envDefault:"sessions"`
}

// WorkerConfigType is download-worker Telegram tokens and upload retries.
type WorkerConfigType struct {
	Tokens           []string      `env:"TOKENS,required"`
	SessionPrefix    string        `env:"SESSION_PREFIX" envDefault:"worker"`
	TempDir          string        `env:"TEMP_DIR"`
	UploadRetries    int           `env:"UPLOAD_RETRIES" envDefault:"3"`
	UploadRetryDelay time.Duration `env:"UPLOAD_RETRY_DELAY" envDefault:"1s"`
}

// ByseConfigType is the Byse file-host API client settings.
type ByseConfigType struct {
	BaseURL       string        `env:"BASE_URL" envDefault:"https://api.byse.sx"`
	APIKey        string        `env:"API_KEY"`
	Timeout       time.Duration `env:"TIMEOUT" envDefault:"30s"`
	UploadTimeout time.Duration `env:"UPLOAD_TIMEOUT" envDefault:"30m"`
}

// ConfigType is the process configuration loaded from environment variables.
type ConfigType struct {
	ApiConfig      ApiConfigType      `envPrefix:"API_"`
	AuthConfig     AuthConfigType     `envPrefix:"AUTH_"`
	MongoConfig    MongoConfigType    `envPrefix:"MONGO_"`
	BotConfig      BotConfigType      `envPrefix:"BOT_"`
	TelegramConfig TelegramConfigType `envPrefix:"TELEGRAM_"`
	WorkerConfig   WorkerConfigType   `envPrefix:"WORKER_"`
	NatsConfig     NatsConfigType     `envPrefix:"NATS_"`
	MinioConfig    MinioConfigType    `envPrefix:"MINIO_"`
	ByseConfig     ByseConfigType     `envPrefix:"BYSE_"`
}
