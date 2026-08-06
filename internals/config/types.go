package config

import "time"

type ApiConfigType struct {
	Listen string `env:"LISTEN" envDefault:":8080"`
}
type RuntimeConfigType struct {
	LogLevel string `env:"LOG_LEVEL"`
	Dev      bool   `env:"DEV"`
}

type MetricsConfigType struct {
	Enabled bool   `env:"METRICS_ENABLED" envDefault:"true"`
	Address string `env:"METRICS_ADDRESS" envDefault:":9090"`
}

type AuthConfigType struct {
	ClientID      string `env:"CLIENT_ID"`
	ClientSecret  string `env:"CLIENT_SECRET"`
	RedirectURL   string `env:"REDIRECT_URL"`
	SecureCookies bool   `env:"SECURE_COOKIES"`
}

type MongoConfigType struct {
	URI             string        `env:"MONGO_URI"`
	DB              string        `env:"MONGO_DB"`
	UsersCollection string        `env:"USERS_COLLECTION" envDefault:"users"`
	MediaCollection string        `env:"MEDIA_COLLECTION" envDefault:"media"`
	PingTimeout     time.Duration `env:"PING_TIMEOUT" envDefault:"5s"`
}

// ConfigType holds all configuration values loaded from environment variables, including API settings, MongoDB connection details, debug options, and logging preferences.
type ConfigType struct {
	ApiConfig     ApiConfigType     `envPrefix:"API_"`
	RuntimeConfig RuntimeConfigType `envPrefix:"RUNTIME_"`
	MetricsConfig MetricsConfigType `envPrefix:"METRICS_"`
	AuthConfig    AuthConfigType    `envPrefix:"AUTH_"`
	MongoConfig   MongoConfigType   `envPrefix:"MONGO_"`
}
