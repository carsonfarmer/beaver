package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
)

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
