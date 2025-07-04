package main

import (
	"context"

	"github.com/ollama/ollama/api"
)

var defaultTools = map[string]Tool{
	"get_weather": weather{},
}

type weather struct{}

func (weather) Name() string { return "get_weather" }
func (weather) Description() api.Tool {
	return api.Tool{
		Function: api.ToolFunction{
			Name:        "get_weather",
			Description: "get current weather description for a city",
		},
	}
}
func (w weather) Run(_ context.Context, call api.ToolCallFunction) (api.Message, error) {
	return api.Message{Content: "it is hot", Role: w.Name()}, nil
}
