package profiles

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInfoBuildsIsolatedCodexPaths(t *testing.T) {
	info := Info("work")
	if !strings.HasSuffix(info.RootDir, filepath.Join(".codex-multi", "profiles", "work")) {
		t.Fatalf("unexpected rootDir: %s", info.RootDir)
	}
	if !strings.HasSuffix(info.CodexHome, filepath.Join(".codex-multi", "profiles", "work", ".codex")) {
		t.Fatalf("unexpected codexHome: %s", info.CodexHome)
	}
	if filepath.Base(info.AuthPath) != "auth.json" {
		t.Fatalf("unexpected authPath: %s", info.AuthPath)
	}
}
