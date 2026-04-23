package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

// Defaults holds default session configuration.
type Defaults struct {
	Model        string `json:"model,omitempty"` // "provider/model"
	ThoughtLevel string `json:"thoughtLevel,omitempty"`
}

// ModelRegistry resolves model IDs to language models and provides defaults.
type ModelRegistry interface {
	ResolveModel(ctx context.Context, model string) (fantasy.LanguageModel, error)
	ModelOptions(model, thoughtLevel string) ModelOptions
	Defaults() Defaults
	AvailableModelGroups() []ModelGroup
}

// Registry is a model registry loaded from a config file.
// Implements ModelRegistry.
type Registry struct {
	Providers map[string]ProviderConfig `json:"providers"`
	defaults  Defaults
}

// LoadRegistry reads and parses a config file into a Registry.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var raw struct {
		Providers map[string]ProviderConfig `json:"providers"`
		Defaults  Defaults                  `json:"defaults,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &Registry{Providers: raw.Providers, defaults: raw.Defaults}, nil
}

// Defaults returns the default session configuration.
func (r *Registry) Defaults() Defaults {
	return r.defaults
}

// ConfigID identifies a session config option.
type ConfigID = acp.SessionConfigID

// Session config option IDs.
const (
	SessionConfigModel        ConfigID = "model"
	SessionConfigThoughtLevel ConfigID = "thought_level"
)

// SessionOptions builds ACP session config options from the registry's
// available models plus the caller's current model and thought-level
// selections. Empty values fall back to registry defaults.
func SessionOptions(registry ModelRegistry, currentModel, currentThoughtLevel string) []acp.SessionConfigOption {
	var modelGroups []acp.SessionConfigSelectGroup
	for _, g := range registry.AvailableModelGroups() {
		var opts []acp.SessionConfigSelectOption
		for _, m := range g.Models {
			opts = append(opts, acp.SessionConfigSelectOption{
				Value: acp.SessionConfigValueID(m.ID),
				Name:  m.Name,
			})
		}
		modelGroups = append(modelGroups, acp.SessionConfigSelectGroup{
			Group:   acp.SessionConfigGroupID(g.Provider),
			Name:    g.Name,
			Options: opts,
		})
	}

	if currentModel == "" {
		currentModel = registry.Defaults().Model
	}
	if currentThoughtLevel == "" {
		currentThoughtLevel = registry.Defaults().ThoughtLevel
	}
	return []acp.SessionConfigOption{
		acp.NewSessionConfigOptionSelect(SessionConfigModel, "Model",
			acp.SessionConfigValueID(currentModel), modelGroups),
		acp.NewSessionConfigOptionSelect(SessionConfigThoughtLevel, "Thought Level",
			acp.SessionConfigValueID(currentThoughtLevel), thoughtSelectOptions),
	}
}
