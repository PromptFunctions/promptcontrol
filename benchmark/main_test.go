package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_InvalidContract(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "invalid.md")
	if err := os.WriteFile(contractPath, []byte(`<contract>
<section name="issue">
</section>
</contract>
`), 0o600); err != nil {
		t.Fatalf("WriteFile invalid contract failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{contractPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}

	output := stderr.String()
	for _, want := range []string{
		"invalid contract",
		"path: " + contractPath,
		"raw XML-like syntax must be wrapped in HTML comments",
		"Contract elements should be wrapped around with <scml> </scml> keys",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stderr missing %q: %q", want, output)
		}
	}
}

func TestResolveAnthropicAPIKeyFallback(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	apiKey, err := resolveAnthropicAPIKey()
	if err != nil {
		t.Fatalf("resolveAnthropicAPIKey failed: %v", err)
	}
	if apiKey != defaultAnthropicAPIKey {
		t.Fatalf("unexpected fallback key: got %q want %q", apiKey, defaultAnthropicAPIKey)
	}
}

func TestResolveAnthropicAPIKeyEnvPrecedence(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-key")

	apiKey, err := resolveAnthropicAPIKey()
	if err != nil {
		t.Fatalf("resolveAnthropicAPIKey failed: %v", err)
	}
	if apiKey != "env-key" {
		t.Fatalf("unexpected env key: got %q want %q", apiKey, "env-key")
	}
}
