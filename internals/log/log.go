package log

import (
	"os"
	"sync"

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
)

var loggerOnce sync.Once

func GetLogger(module LogNS) *zap.Logger {
	return zap.L().Named(string(module))
}

// Named returns a module logger with an additional component name.
func Named(module LogNS, component string) *zap.Logger {
	return GetLogger(module).Named(component)
}

func Setup(dev bool, level string) {
	loggerOnce.Do(func() {
		var llCfg zap.Config
		if dev {
			llCfg = zap.NewDevelopmentConfig()
		} else {
			llCfg = zap.NewProductionConfig()
		}
		ll, err := zap.ParseAtomicLevel(level)
		if err != nil {
			zap.L().With(zap.Error(err)).Error("failed to parse log level. using default")
			return
		}
		llCfg.Level = ll
		logger, err := llCfg.Build(zap.AddStacktrace(zap.ErrorLevel))
		if err != nil {
			zap.L().With(zap.Error(err)).Error("failed to build logger. using default")
			return
		}
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
