package app

import (
	"go.uber.org/zap"
)

func Logger(level string) *zap.Logger {
	cfg := zap.NewProductionConfig()
	// TODO: load from level string
	lg, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return lg
}
