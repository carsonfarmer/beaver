package llm

import "testing"

func TestModelOptions_Limits(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{
		"test": {Variant: VariantOpenAI, Models: map[string]ModelConfig{
			"m1": {Limit: &ModelLimits{Context: 128000, Output: 4096}},
			"m2": {},
		}},
	}}
	opts := reg.ModelOptions("test/m1", "")
	if opts.ContextWindow != 128000 {
		t.Fatalf("expected context 128000, got %d", opts.ContextWindow)
	}
	if opts.MaxOutputTokens == nil || *opts.MaxOutputTokens != 4096 {
		t.Fatalf("expected output 4096, got %v", opts.MaxOutputTokens)
	}
	opts2 := reg.ModelOptions("test/m2", "")
	if opts2.ContextWindow != 0 || opts2.MaxOutputTokens != nil {
		t.Fatal("expected zero limits for m2")
	}
}

func TestModelOptions_ProviderOptions(t *testing.T) {
	variants := []struct {
		name    string
		variant Variant
	}{
		{"openai", VariantOpenAI},
		{"anthropic", VariantAnthropic},
		{"google", VariantGoogle},
		{"openrouter", VariantOpenRouter},
	}
	for _, v := range variants {
		reg := &Registry{Providers: map[string]ProviderConfig{
			v.name: {Variant: v.variant, Models: map[string]ModelConfig{"m1": {}}},
		}}
		opts := reg.ModelOptions(v.name+"/m1", "medium")
		if opts.ProviderOptions == nil {
			t.Fatalf("expected provider options for %s", v.name)
		}
	}
}

func TestModelOptions_EmptyThoughtLevel(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{
		"openai": {Variant: VariantOpenAI, Models: map[string]ModelConfig{"m1": {}}},
	}}
	opts := reg.ModelOptions("openai/m1", "")
	if opts.ProviderOptions != nil {
		t.Fatal("expected nil provider options for empty thought level")
	}
}

func TestModelOptions_UnknownModel(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{}}
	opts := reg.ModelOptions("nope/nope", "high")
	if opts.ContextWindow != 0 || opts.ProviderOptions != nil {
		t.Fatal("expected zero options for unknown model")
	}
}

func TestModelOptions_OpenAICompat(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{
		"compat": {Variant: VariantOpenAICompat, Models: map[string]ModelConfig{"m1": {}}},
	}}
	opts := reg.ModelOptions("compat/m1", "high")
	if opts.ProviderOptions != nil {
		t.Fatal("expected nil provider options for openaicompat")
	}
}
