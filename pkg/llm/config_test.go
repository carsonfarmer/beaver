package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{
		"providers": {
			"test": {
				"variant": "openai",
				"env": ["TEST_KEY"],
				"models": { "m1": { "name": "Model 1" } }
			}
		},
		"defaults": { "model": "test/m1" }
	}`), 0o644)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(reg.Providers))
	}
	if reg.Defaults().Model != "test/m1" {
		t.Fatalf("expected default model %q, got %q", "test/m1", reg.Defaults().Model)
	}
	if reg.Providers["test"].Variant != VariantOpenAI {
		t.Fatalf("expected variant %q, got %q", VariantOpenAI, reg.Providers["test"].Variant)
	}
}

func TestLoadRegistry_NotFound(t *testing.T) {
	_, err := LoadRegistry("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadRegistry_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{invalid`), 0o644)
	_, err := LoadRegistry(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadRegistry_ModelLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{
		"providers": {
			"test": {
				"variant": "openai",
				"env": ["TEST_KEY"],
				"models": {
					"m1": { "limit": { "context": 128000, "output": 4096 } }
				}
			}
		}
	}`), 0o644)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	m := reg.Providers["test"].Models["m1"]
	if m.Limit == nil || m.Limit.Context != 128000 || m.Limit.Output != 4096 {
		t.Fatalf("unexpected limits: %+v", m.Limit)
	}
}

func TestDefaults(t *testing.T) {
	reg := &Registry{defaults: Defaults{Model: "openai/gpt-4.1-mini", ThoughtLevel: "medium"}}
	d := reg.Defaults()
	if d.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("unexpected model %q", d.Model)
	}
	if d.ThoughtLevel != "medium" {
		t.Fatalf("unexpected thought level %q", d.ThoughtLevel)
	}
}
