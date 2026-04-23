package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
)

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

// ResolveModel parses a "provider/model" string and returns a LanguageModel.
func (r *Registry) ResolveModel(ctx context.Context, model string) (fantasy.LanguageModel, error) {
	prov, mc, ok := r.lookup(model)
	if !ok {
		return nil, fmt.Errorf("unknown model %q", model)
	}
	_, modelID, _ := strings.Cut(model, "/")

	variant := prov.Variant
	if mc.Variant != nil {
		variant = *mc.Variant
	}

	var key string
	for _, v := range prov.Env {
		if key = os.Getenv(v); key != "" {
			break
		}
	}
	if key == "" {
		return nil, fmt.Errorf("no API key found (tried %v)", prov.Env)
	}

	fp, err := newProvider(variant, key, prov.API)
	if err != nil {
		return nil, err
	}
	return fp.LanguageModel(ctx, modelID)
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

func (r *Registry) lookup(model string) (ProviderConfig, ModelConfig, bool) {
	providerID, modelID, ok := strings.Cut(model, "/")
	if !ok {
		return ProviderConfig{}, ModelConfig{}, false
	}
	prov, ok := r.Providers[providerID]
	if !ok {
		return ProviderConfig{}, ModelConfig{}, false
	}
	mc, ok := prov.Models[modelID]
	if !ok {
		return ProviderConfig{}, ModelConfig{}, false
	}
	return prov, mc, true
}
