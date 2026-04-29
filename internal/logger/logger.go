package logger

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
	"sync"
)

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
