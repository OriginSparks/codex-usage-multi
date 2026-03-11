package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9._@+-]+$`)

type InfoSet struct {
	Name         string
	RootDir      string
	CodexHome    string
	AuthPath     string
	MetadataPath string
}

type Metadata struct {
	Name          string `json:"name"`
	CreatedAt     string `json:"createdAt"`
	LastCheckedAt string `json:"lastCheckedAt"`
}

func AppRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex-multi"
	}
	return filepath.Join(home, ".codex-multi")
}

func ProfilesRoot() string {
	return filepath.Join(AppRoot(), "profiles")
}

func Info(name string) InfoSet {
	root := filepath.Join(ProfilesRoot(), name)
	codexHome := filepath.Join(root, ".codex")
	return InfoSet{
		Name:         name,
		RootDir:      root,
		CodexHome:    codexHome,
		AuthPath:     filepath.Join(codexHome, "auth.json"),
		MetadataPath: filepath.Join(root, "metadata.json"),
	}
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return errors.New("profile name may only contain letters, numbers, dot, underscore, dash, plus, and at-sign")
	}
	return nil
}

func EnsureRoot() error {
	return os.MkdirAll(ProfilesRoot(), 0o755)
}

func Add(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := EnsureRoot(); err != nil {
		return err
	}

	info := Info(name)
	if err := os.Mkdir(info.RootDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("profile already exists: %s", name)
		}
		return err
	}
	if err := os.MkdirAll(info.CodexHome, 0o755); err != nil {
		return err
	}

	meta := Metadata{
		Name:          name,
		CreatedAt:     nowUTC(),
		LastCheckedAt: "",
	}
	return writeMetadata(info.MetadataPath, meta)
}

func List() ([]string, error) {
	if err := EnsureRoot(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(ProfilesRoot())
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func LoadMetadata(name string) (Metadata, error) {
	data, err := os.ReadFile(Info(name).MetadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{Name: name}, nil
		}
		return Metadata{}, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	if meta.Name == "" {
		meta.Name = name
	}
	return meta, nil
}

func MarkChecked(name string) error {
	meta, err := LoadMetadata(name)
	if err != nil {
		return err
	}
	if meta.Name == "" {
		meta.Name = name
	}
	if meta.CreatedAt == "" {
		meta.CreatedAt = nowUTC()
	}
	meta.LastCheckedAt = nowUTC()
	return writeMetadata(Info(name).MetadataPath, meta)
}

func writeMetadata(path string, meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
