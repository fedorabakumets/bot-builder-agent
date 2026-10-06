package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bot-builder-agent/internal/app"
	"bot-builder-agent/internal/config"
	"bot-builder-agent/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "конфиг: %v\n", err)
		os.Exit(1)
	}
	logg, err := logger.New(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "лог: %v\n", err)
		os.Exit(1)
	}
	defer logg.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg, logg)
	logg.Info("запуск")
	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error("остановка с ошибкой: %v", err)
		os.Exit(1)
	}
}
