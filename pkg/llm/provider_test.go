package llm

import "testing"

func TestNewProvider(t *testing.T) {
	tests := []struct {
		variant Variant
		baseURL string
		wantErr bool
	}{
		{VariantOpenAI, "", false},
		{VariantAnthropic, "", false},
		{VariantOpenRouter, "", false},
		{VariantGoogle, "", false},
		{VariantOpenAICompat, "https://example.com/v1", false},
		{VariantOpenAICompat, "", true},
		{"unknown", "", true},
	}
	for _, tt := range tests {
		_, err := newProvider(tt.variant, "fake-key", tt.baseURL)
		if (err != nil) != tt.wantErr {
			t.Errorf("newProvider(%q, baseURL=%q): err=%v, wantErr=%v", tt.variant, tt.baseURL, err, tt.wantErr)
		}
	}
}
