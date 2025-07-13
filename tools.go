package main

import (
	"context"
	"encoding/json"
	"fmt"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ollama/ollama/api"
	"github.com/twistedogic/serp"
)

type Tool interface {
	Description() api.Tool
	Run(context.Context, api.ToolCallFunction) (api.Message, error)
}

type Function struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

func (f Function) ToolFunction() api.ToolFunction {
	var function api.ToolFunction
	b, _ := json.Marshal(&f)
	json.Unmarshal(b, &function)
	return function
}

func (f Function) Tool() api.Tool {
	return api.Tool{
		Type:     "function",
		Function: f.ToolFunction(),
	}
}

type Parameters struct {
	Type       string               `json:"type"`
	Required   []string             `json:"required,omitempty"`
	Properties map[string]*Property `json:"properties"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

func fromMCPTool(tool *mcp.Tool) (api.Tool, error) {
	openTool := tool.InputSchema
	parameters := Parameters{Required: openTool.Required, Properties: make(map[string]*Property)}
	for name, param := range openTool.Properties {
		enums := make([]string, 0, len(param.Enum))
		for _, e := range param.Enum {
			str, ok := e.(string)
			if !ok {
				return api.Tool{}, fmt.Errorf("toOllamaTools: enum must be string, but got %v", e)
			}
			enums = append(enums, str)
		}
		parameters.Properties[name] = &Property{
			Type:        param.Type,
			Description: param.Description,
			Enum:        enums,
		}
	}
	b, err := json.Marshal(Function{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  parameters,
	})
	if err != nil {
		return api.Tool{}, err
	}
	var function api.ToolFunction
	if err := json.Unmarshal(b, &function); err != nil {
		return api.Tool{}, err
	}
	return api.Tool{
		Type:     "function",
		Function: function,
	}, nil
}

func FromMCPClient(ctx context.Context, client *mcp.ClientSession) ([]api.Tool, error) {
	tools := make([]api.Tool, 0)
	var cursor string
	for {
		res, err := client.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, tool := range res.Tools {
			if err != nil {
				return nil, err
			}
			apiTool, err := fromMCPTool(tool)
			if err != nil {
				return nil, err
			}
			tools = append(tools, apiTool)
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return tools, nil
}

type toolManager struct {
	tools map[string]Tool
}

func NewToolManager() *toolManager {
	return &toolManager{tools: make(map[string]Tool)}
}

func (m *toolManager) AddTool(tools ...Tool) error {
	for _, tool := range tools {
		name := tool.Description().Function.Name
		if _, exist := m.tools[name]; exist {
			return fmt.Errorf("tool with name %q already exists", name)
		}
		m.tools[name] = tool
	}
	return nil
}

func (m *toolManager) List() []api.Tool {
	tools := make([]api.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool.Description())
	}
	return tools
}

func (m *toolManager) Call(ctx context.Context, c api.ToolCall) (api.Message, error) {
	tool, exist := m.tools[c.Function.Name]
	if !exist {
		return api.Message{}, fmt.Errorf("called non-exist tool %q", c.Function.Name)
	}
	return tool.Run(ctx, c.Function)
}

var defaultTools = NewToolManager()

type serper struct {
	client serp.Serper
}

func NewSerper() Tool {
	return serper{client: serp.New()}
}

func (s serper) Description() api.Tool {
	return Function{
		Name:        "web_search",
		Description: "Perform web search for provided search_term and return response as markdown.",
		Parameters: Parameters{
			Type:     "object",
			Required: []string{"search_term"},
			Properties: map[string]*Property{
				"search_term": &Property{
					Type:        "string",
					Description: "term to search for web results",
				},
			},
		},
	}.Tool()
}

func (s serper) Run(ctx context.Context, call api.ToolCallFunction) (api.Message, error) {
	val, ok := call.Arguments["search_term"]
	if !ok {
		return api.Message{}, fmt.Errorf("`search_term` not provided")
	}
	term, ok := val.(string)
	if !ok {
		return api.Message{}, fmt.Errorf("search_term expect a string, but got %v")
	}
	result, err := s.client.Define(ctx, term)
	if err != nil {
		return api.Message{}, err
	}
	md, err := htmltomarkdown.ConvertString(string(result))
	if err != nil {
		return api.Message{}, err
	}
	return api.Message{Role: call.Name, Content: md}, nil
}
