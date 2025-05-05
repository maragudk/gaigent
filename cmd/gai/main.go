package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"

	"maragu.dev/env"
	anthropic "maragu.dev/gai-anthropic"

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

	c := anthropic.NewClient(anthropic.NewClientOptions{
		Key: env.GetStringOrDefault("ANTHROPIC_KEY", ""),
		Log: log,
	})

	cc := c.NewChatCompleter(anthropic.NewChatCompleterOptions{
		Model: anthropic.ChatCompleteModelClaude3_7SonnetLatest,
	})

	agent := gaigent.NewAgent(gaigent.NewAgentOptions{
		ChatCompleter: cc,
		Log:           log,
	})

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	return agent.Run(context.Background(), getUserMessage, os.Stdout)
}
