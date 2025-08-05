package logging

import "go.uber.org/zap"

var logger *zap.SugaredLogger

func Init() {
	raw, _ := zap.NewDevelopment()
	logger = raw.Sugar()
}

func L() *zap.SugaredLogger {
	return logger
}

func Sync() {
	logger.Sync()
}
