package llm

import (
	"fmt"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
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

// ProviderConfig defines an API provider that hosts models.
type ProviderConfig struct {
	Name    string                 `json:"name,omitempty"`
	Variant Variant                `json:"variant"`
	API     string                 `json:"api,omitempty"` // base URL override (works for all variants)
	Env     []string               `json:"env"`           // env var names for API key
	Models  map[string]ModelConfig `json:"models"`
}

func newProvider(v Variant, key, baseURL string) (fantasy.Provider, error) {
	switch v {
	case VariantOpenAI:
		opts := []openai.Option{openai.WithAPIKey(key), openai.WithUseResponsesAPI()}
		if baseURL != "" {
			opts = append(opts, openai.WithBaseURL(baseURL))
		}
		return openai.New(opts...)
	case VariantAnthropic:
		opts := []anthropic.Option{anthropic.WithAPIKey(key)}
		if baseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(baseURL))
		}
		return anthropic.New(opts...)
	case VariantOpenRouter:
		opts := []openrouter.Option{openrouter.WithAPIKey(key)}
		return openrouter.New(opts...)
	case VariantGoogle:
		opts := []google.Option{google.WithGeminiAPIKey(key)}
		if baseURL != "" {
			opts = append(opts, google.WithBaseURL(baseURL))
		}
		return google.New(opts...)
	case VariantOpenAICompat:
		if baseURL == "" {
			return nil, fmt.Errorf("openaicompat variant requires an api base URL")
		}
		return openaicompat.New(openaicompat.WithAPIKey(key), openaicompat.WithBaseURL(baseURL))
	default:
		return nil, fmt.Errorf("unknown variant %q", v)
	}
}
