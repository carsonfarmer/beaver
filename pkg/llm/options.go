package llm

import (
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
)

// ModelOptions holds all call-time settings for a model.
type ModelOptions struct {
	ContextWindow   int64
	MaxOutputTokens *int64
	ProviderOptions fantasy.ProviderOptions
}

// ModelOptions returns call-time settings for the given model and thought level.
func (r *Registry) ModelOptions(model, thoughtLevel string) ModelOptions {
	prov, mc, ok := r.lookup(model)
	if !ok {
		return ModelOptions{}
	}
	variant := prov.Variant
	if mc.Variant != nil {
		variant = *mc.Variant
	}
	opts := ModelOptions{
		ProviderOptions: buildProviderOptions(variant, thoughtLevel),
	}
	if mc.Limit != nil {
		opts.ContextWindow = mc.Limit.Context
		if mc.Limit.Output > 0 {
			opts.MaxOutputTokens = &mc.Limit.Output
		}
	}
	return opts
}

func buildProviderOptions(variant Variant, thoughtLevel string) fantasy.ProviderOptions {
	if thoughtLevel == "" {
		return nil
	}
	t := true
	switch variant {
	case VariantOpenAI:
		effort := openai.ReasoningEffort(thoughtLevel)
		return fantasy.ProviderOptions{
			openai.Name: &openai.ProviderOptions{ReasoningEffort: &effort},
		}
	case VariantAnthropic:
		effort := anthropic.Effort(thoughtLevel)
		return fantasy.ProviderOptions{
			anthropic.Name: &anthropic.ProviderOptions{
				Effort:        &effort,
				SendReasoning: &t,
			},
		}
	case VariantGoogle:
		level := strings.ToUpper(thoughtLevel)
		return fantasy.ProviderOptions{
			google.Name: &google.ProviderOptions{
				ThinkingConfig: &google.ThinkingConfig{
					ThinkingLevel:   &level,
					IncludeThoughts: &t,
				},
			},
		}
	case VariantOpenRouter:
		effort := openrouter.ReasoningEffort(thoughtLevel)
		return fantasy.ProviderOptions{
			openrouter.Name: &openrouter.ProviderOptions{
				Reasoning: &openrouter.ReasoningOptions{
					Enabled: &t,
					Effort:  &effort,
				},
			},
		}
	default:
		return nil
	}
}
