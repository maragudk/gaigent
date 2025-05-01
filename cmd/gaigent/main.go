package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"maragu.dev/env"

	"maragu.dev/gaigent"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("Error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	_ = env.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	agent := gaigent.NewAgent(gaigent.NewAgentOptions{
		Key: env.GetStringOrDefault("ANTHROPIC_KEY", ""),
		Log: log,
	})

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	return agent.Run(ctx, getUserMessage, os.Stdout)
}
