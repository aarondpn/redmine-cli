package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	promptfs "github.com/aarondpn/redmine-cli/v2/prompts"
)

type promptDefinition struct {
	Prompt   *mcp.Prompt
	Template *template.Template
}

type promptFrontMatter struct {
	Name        string             `yaml:"name"`
	Title       string             `yaml:"title"`
	Description string             `yaml:"description"`
	Arguments   []promptArgumentFM `yaml:"arguments"`
}

type promptArgumentFM struct {
	Name        string `yaml:"name"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

func loadPromptDefinitions() map[string]promptDefinition {
	definitions, err := parsePromptDefinitions(promptfs.FS())
	if err != nil {
		panic(err)
	}
	return definitions
}

func parsePromptDefinitions(fsys fs.FS) (map[string]promptDefinition, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read prompts: %w", err)
	}

	definitions := make(map[string]promptDefinition, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read prompt %s: %w", entry.Name(), err)
		}
		def, err := parsePromptDefinition(entry.Name(), string(body))
		if err != nil {
			return nil, err
		}
		if _, exists := definitions[def.Prompt.Name]; exists {
			return nil, fmt.Errorf("duplicate prompt name %q", def.Prompt.Name)
		}
		definitions[def.Prompt.Name] = def
	}

	return definitions, nil
}

func parsePromptDefinition(name, raw string) (promptDefinition, error) {
	metaText, templateText, err := splitFrontMatter(raw)
	if err != nil {
		return promptDefinition{}, fmt.Errorf("parse prompt %s: %w", name, err)
	}

	var meta promptFrontMatter
	if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
		return promptDefinition{}, fmt.Errorf("decode prompt %s frontmatter: %w", name, err)
	}
	if meta.Name == "" {
		return promptDefinition{}, fmt.Errorf("prompt %s missing name", name)
	}

	args := make([]*mcp.PromptArgument, 0, len(meta.Arguments))
	for _, arg := range meta.Arguments {
		if arg.Name == "" {
			return promptDefinition{}, fmt.Errorf("prompt %s has argument with empty name", meta.Name)
		}
		args = append(args, &mcp.PromptArgument{
			Name:        arg.Name,
			Title:       arg.Title,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}

	tmpl, err := template.New(meta.Name).Option("missingkey=error").Parse(strings.TrimSpace(templateText))
	if err != nil {
		return promptDefinition{}, fmt.Errorf("parse prompt template %s: %w", meta.Name, err)
	}

	return promptDefinition{
		Prompt: &mcp.Prompt{
			Name:        meta.Name,
			Title:       meta.Title,
			Description: meta.Description,
			Arguments:   args,
		},
		Template: tmpl,
	}, nil
}

func splitFrontMatter(raw string) (string, string, error) {
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", fmt.Errorf("prompt file missing opening frontmatter delimiter")
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", "", fmt.Errorf("prompt file missing closing frontmatter delimiter")
	}
	return rest[:idx], rest[idx+5:], nil
}

func registerPrompts(s *mcp.Server) {
	for _, def := range loadPromptDefinitions() {
		definition := def
		s.AddPrompt(definition.Prompt, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			args := map[string]string{}
			if req.Params != nil && req.Params.Arguments != nil {
				for key, value := range req.Params.Arguments {
					args[key] = value
				}
			}
			for _, arg := range definition.Prompt.Arguments {
				if arg.Required && strings.TrimSpace(args[arg.Name]) == "" {
					return nil, fmt.Errorf("missing required prompt argument %q", arg.Name)
				}
			}

			var rendered bytes.Buffer
			if err := definition.Template.Execute(&rendered, args); err != nil {
				return nil, fmt.Errorf("render prompt %s: %w", definition.Prompt.Name, err)
			}

			return &mcp.GetPromptResult{
				Description: definition.Prompt.Description,
				Messages: []*mcp.PromptMessage{{
					Role:    mcp.Role("user"),
					Content: &mcp.TextContent{Text: rendered.String()},
				}},
			}, nil
		})
	}
}
