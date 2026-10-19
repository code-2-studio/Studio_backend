package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func init() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	l, err := config.Build()
	if err != nil {
		l, _ = zap.NewDevelopment()
	}
	Logger = l.Sugar()
}

// Info logs messages at level info
func Info(msg string, keysAndValues ...interface{}) {
	Logger.Infow(msg, keysAndValues...)
}

// Error logs messages at level error
func Error(msg string, keysAndValues ...interface{}) {
	Logger.Errorw(msg, keysAndValues...)
}

// Debug logs messages at level debug
func Debug(msg string, keysAndValues ...interface{}) {
	Logger.Debugw(msg, keysAndValues...)
}

// Warn logs messages at level warn
func Warn(msg string, keysAndValues ...interface{}) {
	Logger.Warnw(msg, keysAndValues...)
}
