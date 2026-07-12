package log

import (
	"os"
	"sync"

	godotenv "github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogNS string

const (
	API      LogNS = "api"
	CMD      LogNS = "cmd"
	CONFIG   LogNS = "config"
	LOG      LogNS = "log"
	TELEGRAM LogNS = "telegram"
	BOT      LogNS = "bot"
	STREAM   LogNS = "stream"
)

var loggerOnce sync.Once

func GetLogger(module LogNS) *zap.Logger {
	return zap.L().Named(string(module))
}

// Named returns a module logger with an additional component name.
func Named(module LogNS, component string) *zap.Logger {
	return GetLogger(module).Named(component)
}

func Setup() {
	loggerOnce.Do(func() {
		// Load .env early so logger config can read env vars.
		if _, err := os.Stat(".env"); err == nil {
			_ = godotenv.Load()
		}

		var llCfg zap.Config
		if os.Getenv("RUNTIME_DEV") == "true" || os.Getenv("RUNTIME_DEV") == "1" {
			llCfg = zap.NewDevelopmentConfig()
		} else {
			llCfg = zap.NewProductionConfig()
		}
		levelStr := os.Getenv("RUNTIME_LOG_LEVEL")

		if levelStr != "" {
			level, err := zap.ParseAtomicLevel(levelStr)
			if err != nil {
				zap.L().Warn("can not parse log level; using default", zap.String("level", levelStr), zap.String("default", llCfg.Level.String()), zap.Error(err))
			} else {
				llCfg.Level = level
			}
		}
		logger, _ := llCfg.Build(zap.AddStacktrace(zap.ErrorLevel))
		zap.ReplaceGlobals(logger)
	})
}

type LogSinkEnum string

const (
	LogSyncEnumNone   LogSinkEnum = "none"
	LogSyncEnumStdout LogSinkEnum = ""
)

func NewLogger(sink LogSinkEnum, level zapcore.Level) *zap.Logger {
	var sinker zapcore.WriteSyncer
	ll := GetLogger(LOG)
	switch sink {
	case LogSyncEnumNone:
		ll.Info("log sink disabled")
		return zap.NewNop()
	case LogSyncEnumStdout:
		sinker = zapcore.AddSync(os.Stdout)
		ll.Info("log sink stdout")
	default:
		f, err := os.OpenFile(string(sink), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			ll.With(zap.Error(err)).Error("failed to open log file, falling back to stdout", zap.String("path", string(sink)))
		} else {
			ll.With(zap.String("path", string(sink))).Info("opened log file")
			sinker = zapcore.AddSync(f)
		}
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		sinker,
		level,
	)
	return zap.New(core)
}
