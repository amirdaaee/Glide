package config

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

// ConfigType holds all configuration values loaded from environment variables, including API settings, MongoDB connection details, debug options, and logging preferences.
type ConfigType struct {
	ApiConfig     ApiConfigType     `envPrefix:"API_"`
	RuntimeConfig RuntimeConfigType `envPrefix:"RUNTIME_"`
	MetricsConfig MetricsConfigType `envPrefix:"METRICS_"`
}
