package gaigent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"maragu.dev/gai"
	"maragu.dev/gai/tools"
)

type Agent struct {
	cc  gai.ChatCompleter
	log *slog.Logger
}

type NewAgentOptions struct {
	ChatCompleter gai.ChatCompleter
	Log           *slog.Logger
}

func NewAgent(opts NewAgentOptions) *Agent {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	return &Agent{
		cc:  opts.ChatCompleter,
		log: opts.Log,
	}
}

func (a *Agent) Run(ctx context.Context, getUserMessage func() (string, bool), out io.Writer) error {
	var conversation []gai.Message

	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	rootFS := root.FS()

	tools := []gai.Tool{
		tools.NewReadFile(rootFS),
		tools.NewListDir(rootFS),
		tools.NewGetTime(time.Now),
	}

	readUserInput := true
	for {
		if readUserInput {
			_, _ = fmt.Fprint(out, "\n\u001b[1;94mMe\u001b[0m: ")
			userInput, ok := getUserMessage()
			if !ok {
				break
			}

			userMessage := gai.NewUserTextMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		res, err := a.cc.ChatComplete(ctx, gai.ChatCompleteRequest{
			Messages: conversation,
			Tools:    tools,
		})
		if err != nil {
			return err
		}

		var turn string
		var parts []gai.MessagePart
		var toolResult gai.ToolResult
		for part, err := range res.Parts() {
			if err != nil {
				return err
			}

			switch part.Type {
			case gai.MessagePartTypeText:
				if turn != "agent" {
					_, _ = fmt.Fprint(out, "\n\u001b[1;92mGAI\u001b[0m: ")
					turn = "agent"
				}
				_, _ = fmt.Fprint(out, part.Text())

			case gai.MessagePartTypeToolCall:
				if turn != "tool" {
					_, _ = fmt.Fprint(out, "\n\u001b[0;32mTool\u001b[0m: ")
					turn = "tool"
				}
				toolCall := part.ToolCall()

				for _, tool := range tools {
					if tool.Name == toolCall.Name {
						_, _ = fmt.Fprintf(out, "\u001b[1;37m%v\u001b[0m(%v)", toolCall.Name, string(toolCall.Args))
						result, err := tool.Function(ctx, toolCall.Args)
						toolResult = gai.ToolResult{
							ID:      toolCall.ID,
							Content: result,
							Err:     err,
						}
						break
					}
				}
			default:
				panic(fmt.Sprintf("unknown message part type: %v", part.Type))
			}

			parts = append(parts, part)
		}

		conversation = append(conversation, gai.Message{
			Role:  gai.MessageRoleAssistant,
			Parts: parts,
		})

		if toolResult.ID != "" {
			conversation = append(conversation, gai.NewUserToolResultMessage(toolResult))
			readUserInput = false
			continue
		}

		_, _ = fmt.Fprintln(out)
		readUserInput = true
	}

	return nil
}
