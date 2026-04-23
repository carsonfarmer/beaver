package llm

import (
	"context"
	"testing"
)

func TestResolveModel_InvalidFormat(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{}}
	_, err := reg.ResolveModel(context.Background(), "no-slash")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveModel_UnknownProvider(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{}}
	_, err := reg.ResolveModel(context.Background(), "nope/model")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveModel_UnknownModel(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{
		"test": {Variant: VariantOpenAI, Env: []string{"TEST_KEY"}, Models: map[string]ModelConfig{}},
	}}
	_, err := reg.ResolveModel(context.Background(), "test/nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveModel_NoAPIKey(t *testing.T) {
	reg := &Registry{Providers: map[string]ProviderConfig{
		"test": {Variant: VariantOpenAI, Env: []string{"NONEXISTENT_KEY_12345"}, Models: map[string]ModelConfig{"m1": {}}},
	}}
	_, err := reg.ResolveModel(context.Background(), "test/m1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveModel_OpenAICompatNoBaseURL(t *testing.T) {
	t.Setenv("TEST_KEY_COMPAT", "fake-key")
	reg := &Registry{Providers: map[string]ProviderConfig{
		"test": {Variant: VariantOpenAICompat, Env: []string{"TEST_KEY_COMPAT"}, Models: map[string]ModelConfig{"m1": {}}},
	}}
	_, err := reg.ResolveModel(context.Background(), "test/m1")
	if err == nil {
		t.Fatal("expected error")
	}
}
