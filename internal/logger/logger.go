package logger

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
	"sync"
)

type ctxKey struct{}

var once sync.Once

var logger *zap.Logger

func GetLogger() *zap.Logger {
	once.Do(func() {
		stdout := zapcore.AddSync(os.Stdout)
		level := zap.InfoLevel
		levelEnv := os.Getenv("LOG_LEVEL")
		if levelEnv != "" {
			levelFromEnv, err := zapcore.ParseLevel(levelEnv)
			if err != nil {
				log.Println(fmt.Errorf("unable to parse log level from environment variable: %w. Defaulting to INFO", err))
			}
			level = levelFromEnv
		}
		logLevel := zap.NewAtomicLevelAt(level)
		prodConfig := zap.NewProductionEncoderConfig()
		prodConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		encoder := zapcore.NewConsoleEncoder(prodConfig)

		core := zapcore.NewCore(encoder, stdout, logLevel)

		logger = zap.New(core)
	})
	return logger
}

//goland:noinspection GoUnusedExportedFunction,GoUnusedExportedFunction
func FromContext(ctx context.Context) *zap.Logger {
	if zapLogger, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		return zapLogger
	} else if zapLogger := logger; zapLogger != nil {
		return zapLogger
	}
	return zap.NewNop()
}

//goland:noinspection GoUnusedExportedFunction
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	if contextKey, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		if contextKey == l {
			return ctx
		}
	}
	return context.WithValue(ctx, ctxKey{}, l)
}
