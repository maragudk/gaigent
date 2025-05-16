package gaigent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
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

	db := &memoriesDB{}

	tools := []gai.Tool{
		tools.NewEditFile(root),
		tools.NewGetMemories(db),
		tools.NewGetTime(time.Now),
		tools.NewListDir(root),
		tools.NewReadFile(root),
		tools.NewSaveMemory(db),
		tools.NewFetch(nil),
		tools.NewExec(),
	}

	allowedTools := []string{
		"fetch",
		"get_memories",
		"get_time",
		"list_dir",
		"read_file",
		"save_memory",
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
			System:   gai.Ptr("You are an assistant called GAI (pronounced guy). You respond to user requests and use tools to complete tasks. You MUST not mention what tools you have available, just use them when appropriate."),
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

						msg := "y"
						if !slices.Contains(allowedTools, tool.Name) {
							_, _ = fmt.Fprintf(out, "\nAllow [y/n/a]?: ")
							var ok bool
							msg, ok = getUserMessage()
							if !ok {
								return nil
							}
						}

						switch msg := strings.ToLower(msg); msg {
						case "y", "yes", "a", "all":
							if msg == "a" || msg == "all" {
								allowedTools = append(allowedTools, tool.Name)
							}

							result, err := tool.Function(ctx, toolCall.Args)
							toolResult = gai.ToolResult{
								ID:      toolCall.ID,
								Content: result,
								Err:     err,
							}
						case "n", "no":
							toolResult = gai.ToolResult{
								ID:  toolCall.ID,
								Err: errors.New("tool call denied"),
							}
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
			Role:  gai.MessageRoleModel,
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

type memoriesDB struct {
}

func (m *memoriesDB) GetMemories(ctx context.Context) ([]string, error) {
	var memories []string
	data, err := os.ReadFile("memory.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &memories); err != nil {
		return nil, err
	}
	return memories, nil
}

func (m *memoriesDB) SaveMemory(ctx context.Context, memory string) error {
	memories, err := m.GetMemories(ctx)
	if err != nil {
		return err
	}
	memories = append(memories, memory)
	data, err := json.Marshal(memories)
	if err != nil {
		return err
	}
	return os.WriteFile("memory.json", data, 0644)
}
