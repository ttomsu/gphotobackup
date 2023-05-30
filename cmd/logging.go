package cmd

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
)

func NewLogger() *zap.SugaredLogger {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		log.Fatalf("Error establishing logger: %v", err)
	}
	return logger.Sugar()
}
