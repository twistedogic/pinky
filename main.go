package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/ollama/ollama/api"
)

type Tool interface {
	Name() string
	Description() api.Tool
	Run(context.Context, api.ToolCallFunction) (api.Message, error)
}

func printMessage(m api.Message) string {
	content := m.Content
	calls := make([]string, 0, len(m.ToolCalls))
	for _, c := range m.ToolCalls {
		if b, err := json.MarshalIndent(c.Function, "", "  "); err == nil {
			calls = append(calls, "```json\n"+string(b)+"\n```")
		}
	}
	if len(calls) != 0 {
		content = strings.Join(calls, "\n\n")
	}
	return content
}

type Brain struct {
	model   string
	limit   int
	client  *api.Client
	history []api.Message
	tools   map[string]Tool
}

func NewBrain(model string, limit int) (*Brain, error) {
	client, err := api.ClientFromEnvironment()
	return &Brain{
		model:  model,
		limit:  limit,
		client: client,
		tools:  defaultTools,
	}, err
}

func (b *Brain) prompt() error {
	var content string
	fields := make([]huh.Field, 0, len(b.history))
	for _, m := range b.history {
		fields = append(fields, huh.NewNote().Title(m.Role).Description(printMessage(m)))
	}
	fields = append(fields, huh.NewText().Title("prompt").Value(&content).Validate(func(s string) error {
		if len(s) == 0 {
			return fmt.Errorf("`prompt` cannot be empty.")
		}
		return nil
	}))
	if err := huh.NewForm(
		huh.NewGroup(
			fields...,
		),
	).Run(); err != nil {
		return err
	}
	b.history = append(b.history, api.Message{
		Role:    "user",
		Content: content,
	})
	return nil
}

func (b *Brain) show() {
	var content strings.Builder
	for _, m := range b.history {
		content.WriteString("# " + m.Role + "\n")
		content.WriteString(printMessage(m) + "\n\n")
	}
	if md, err := glamour.Render(content.String(), "dark"); err == nil {
		fmt.Println(md)
	} else {
		fmt.Println(content.String())
	}
}

func (b *Brain) request() *api.ChatRequest {
	tools := make(api.Tools, 0, len(b.tools))
	for _, t := range b.tools {
		tools = append(tools, t.Description())
	}
	think := true
	stream := false
	return &api.ChatRequest{
		Model:    b.model,
		Messages: b.history,
		Tools:    tools,
		Think:    &think,
		Stream:   &stream,
	}
}

func (b *Brain) callTools(ctx context.Context, calls []api.ToolCall) error {
	responses := make([]api.Message, len(calls))
	for i, c := range calls {
		tool, exist := b.tools[c.Function.Name]
		if !exist {
			return fmt.Errorf("called non-exist tool %q", c.Function.Name)
		}
		res, err := tool.Run(ctx, c.Function)
		if err != nil {
			return err
		}
		responses[i] = res
	}
	b.history = append(b.history, responses...)
	return nil
}

func (b *Brain) chat(ctx context.Context) error {
	var message api.Message
	if err := b.client.Chat(ctx, b.request(), func(cr api.ChatResponse) error {
		message = cr.Message
		return nil
	}); err != nil {
		return err
	}
	b.history = append(b.history, message)
	return nil
}

func (b *Brain) loop(ctx context.Context) error {
	latest := b.history[len(b.history)-1]
	switch {
	case len(latest.ToolCalls) != 0:
		if err := b.callTools(ctx, latest.ToolCalls); err != nil {
			return err
		}
	default:
		if err := b.chat(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *Brain) start(ctx context.Context) error {
	for len(b.history) <= b.limit || b.limit == 0 {
		latest := b.history[len(b.history)-1]
		if len(latest.ToolCalls) == 0 && strings.ToLower(latest.Role) == "assistant" {
			if err := b.prompt(); err != nil {
				return err
			}
		} else {
			if err := spinner.New().Title(
				"thinking...",
			).Context(ctx).ActionWithErr(b.loop).Run(); err != nil {
				return err
			}
		}
	}
	b.show()
	return nil
}

func (b *Brain) Start() error {
	var systemPrompt string
	var prompt string
	limit := strconv.Itoa(b.limit)
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Inline(true).Title("model").Value(&b.model),
			huh.NewInput().Inline(true).Title("history limit").Value(&limit).Validate(func(s string) error {
				l, err := strconv.Atoi(s)
				if err != nil {
					return err
				}
				b.limit = l
				return nil
			}),
			huh.NewInput().Title("system").Value(&systemPrompt),
			huh.NewText().Title("prompt").Value(&prompt).Validate(func(s string) error {
				if len(s) == 0 {
					return fmt.Errorf("`prompt` cannot be empty.")
				}
				return nil
			}),
		),
	).Run(); err != nil {
		return err
	}
	b.history = append(
		b.history,
		api.Message{Role: "system", Content: systemPrompt},
		api.Message{Role: "user", Content: prompt},
	)

	return b.start(context.Background())
}

func main() {
	brain, err := NewBrain("qwen3", 0)
	if err != nil {
		log.Fatal(err)
	}
	if err := brain.Start(); err != nil {
		log.Fatal(err)
	}
}
