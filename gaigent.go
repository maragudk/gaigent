package gaigent

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"maragu.dev/gai"
	anthropic "maragu.dev/gai-anthropic"
)

type Agent struct {
	c   *anthropic.Client
	cc  gai.ChatCompleter
	log *slog.Logger
}

type NewAgentOptions struct {
	Key string
	Log *slog.Logger
}

func NewAgent(opts NewAgentOptions) *Agent {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	c := anthropic.NewClient(anthropic.NewClientOptions{
		Key: opts.Key,
		Log: opts.Log,
	})

	cc := c.NewChatCompleter(anthropic.NewChatCompleterOptions{
		Model: anthropic.ChatCompleteModelClaude3_7SonnetLatest,
	})

	return &Agent{
		c:   c,
		cc:  cc,
		log: opts.Log,
	}
}

func (a *Agent) Run(ctx context.Context, getUserMessage func() (string, bool), out io.Writer) error {
	var conversation []gai.Message

	readUserInput := true
	for {
		if readUserInput {
			fmt.Fprint(out, "\u001b[94mYou\u001b[0m: ")
			userInput, ok := getUserMessage()
			if !ok {
				break
			}

			userMessage := gai.NewUserTextMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		res, err := a.cc.ChatComplete(ctx, gai.ChatCompleteRequest{
			Messages: conversation,
		})
		if err != nil {
			return err
		}

		fmt.Fprint(out, "\u001b[93mAgent\u001b[0m: ")
		var parts []gai.MessagePart
		for part, err := range res.Parts() {
			if err != nil {
				return err
			}

			switch part.Type {
			case gai.MessagePartTypeText:
				fmt.Fprint(out, part.Text())
			default:
				panic(fmt.Sprintf("unknown message part type: %v", part.Type))
			}

			parts = append(parts, part)
		}
		fmt.Fprintln(out)

		conversation = append(conversation, gai.Message{
			Role:  gai.MessageRoleAssistant,
			Parts: parts,
		})
	}

	return nil
}
