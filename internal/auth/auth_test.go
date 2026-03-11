package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBearerTokenFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	content := `{"auth_mode":"chatgpt","tokens":{"default":{"access_token":"` + strings.Repeat("x", 40) + `"}}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	token, mode, err := ExtractBearerTokenFromFile(path)
	if err != nil {
		t.Fatalf("extract token: %v", err)
	}
	if mode != "chatgpt" {
		t.Fatalf("mode = %q", mode)
	}
	if token != strings.Repeat("x", 40) {
		t.Fatalf("token mismatch")
	}
}

func TestRejectsAPIKeyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(`{"OPENAI_API_KEY":"sk-test","tokens":{}}`), 0o644); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	_, _, err := ExtractBearerTokenFromFile(path)
	if err == nil || !strings.Contains(err.Error(), "API-key based") {
		t.Fatalf("unexpected error: %v", err)
	}
}
