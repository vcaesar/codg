// Package demo provides an example built-in plugin for Codg.
//
// This plugin demonstrates the plugin system's capabilities by
// registering a custom tool, injecting environment variables, and
// appending to the system prompt. It is intended as both a reference
// implementation and a quick-start template for plugin authors.
//
// To enable this demo plugin, add the following to your codg.toml:
//
//	[plugins.demo]
//	type = "builtin"
//	source = "demo"
//
// The plugin registers the following:
//
//   - **hello tool**: A simple tool the LLM can call that echoes a
//     greeting message.
//   - **shell.env hook**: Injects CODG_PLUGIN_DEMO=1 into every shell
//     command execution.
//   - **system.prompt.transform hook**: Appends a short note to the
//     system prompt identifying that the demo plugin is active.
package demo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vcaesar/codg/plugin"
)

func init() {
	plugin.RegisterBuiltin("demo", &Plugin{})
}

// Plugin implements the demo built-in plugin.
type Plugin struct{}

// Init initializes the demo plugin and returns its hooks.
func (p *Plugin) Init(_ context.Context, input plugin.PluginInput) (*plugin.Hooks, error) {
	return &plugin.Hooks{
		Tools: map[string]plugin.ToolDefinition{
			"hello": {
				Description: "A demo greeting tool. Returns a personalized hello message.",
				Parameters: map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name to greet.",
					},
				},
				Required: []string{"name"},
				Execute:  executeHello,
			},
		},
		ShellEnv: func(_ context.Context, _ plugin.ShellEnvInput, output *plugin.ShellEnvOutput) error {
			if output.Env == nil {
				output.Env = make(map[string]string)
			}
			output.Env["CODG_PLUGIN_DEMO"] = "1"
			return nil
		},
		SystemPromptTransform: func(_ context.Context, _ plugin.SystemPromptInput, output *plugin.SystemPromptOutput) error {
			output.System = append(output.System, "The demo plugin is active. You have access to the `hello` tool.")
			return nil
		},
	}, nil
}

// helloArgs is the typed input for the hello tool.
type helloArgs struct {
	Name string `json:"name"`
}

// executeHello implements the hello tool.
func executeHello(_ context.Context, input string) (string, error) {
	var args helloArgs
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	return fmt.Sprintf("Hello, %s! 👋 This message is from the Codg demo plugin.", args.Name), nil
}
