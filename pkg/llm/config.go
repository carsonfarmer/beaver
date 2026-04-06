package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"charm.land/fantasy"
)

// Variant is the API wire protocol type.
type Variant string

const (
	VariantOpenAI       Variant = "openai"
	VariantAnthropic    Variant = "anthropic"
	VariantOpenAICompat Variant = "openaicompat"
	VariantOpenRouter   Variant = "openrouter"
	VariantGoogle       Variant = "google"
)

// Defaults holds default session configuration.
type Defaults struct {
	Model        string `json:"model,omitempty"`        // "provider/model"
	ThoughtLevel string `json:"thoughtLevel,omitempty"`
}

// ModelOptions holds all call-time settings for a model.
type ModelOptions struct {
	ContextWindow   int64
	MaxOutputTokens *int64
	ProviderOptions fantasy.ProviderOptions
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID   string // "provider/model"
	Name string
}

// ModelGroup holds models grouped by provider.
type ModelGroup struct {
	Provider string
	Name     string
	Models   []ModelInfo
}

// ModelRegistry resolves model IDs to language models and provides defaults.
type ModelRegistry interface {
	ResolveModel(ctx context.Context, model string) (fantasy.LanguageModel, error)
	ModelOptions(model, thoughtLevel string) ModelOptions
	Defaults() Defaults
	AvailableModelGroups() []ModelGroup
}

// ProviderConfig defines an API provider that hosts models.
type ProviderConfig struct {
	Name    string                 `json:"name,omitempty"`
	Variant Variant                `json:"variant"`
	API     string                 `json:"api,omitempty"` // base URL override (works for all variants)
	Env     []string               `json:"env"`           // env var names for API key
	Models  map[string]ModelConfig `json:"models"`
}

// ModelConfig defines a model available through a provider.
type ModelConfig struct {
	Name    string       `json:"name,omitempty"`
	Variant *Variant     `json:"variant,omitempty"` // override provider variant
	Limit   *ModelLimits `json:"limit,omitempty"`
}

// ModelLimits defines token limits for a model.
type ModelLimits struct {
	Context int64 `json:"context,omitempty"`
	Output  int64 `json:"output,omitempty"`
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

// AvailableModelGroups returns models grouped by provider.
func (r *Registry) AvailableModelGroups() []ModelGroup {
	var groups []ModelGroup
	for provID, prov := range r.Providers {
		name := prov.Name
		if name == "" {
			name = provID
		}
		var models []ModelInfo
		for modelID, mc := range prov.Models {
			mname := mc.Name
			if mname == "" {
				mname = modelID
			}
			models = append(models, ModelInfo{ID: provID + "/" + modelID, Name: mname})
		}
		if len(models) > 0 {
			groups = append(groups, ModelGroup{Provider: provID, Name: name, Models: models})
		}
	}
	return groups
}
