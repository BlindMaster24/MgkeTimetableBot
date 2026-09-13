package parity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadSurface(path string) (Surface, error) {
	var surface Surface
	if err := readJSON(path, &surface); err != nil {
		return Surface{}, err
	}
	return surface, nil
}

func SaveSurface(path string, surface Surface) error {
	surface = normalizeSurface(surface)
	return writeJSON(path, surface)
}

func LoadAllowlist(path string) (Allowlist, error) {
	var allow Allowlist
	if err := readJSON(path, &allow); err != nil {
		if os.IsNotExist(err) {
			return Allowlist{}, nil
		}
		return Allowlist{}, err
	}
	for _, decision := range allow.Decisions {
		if strings.TrimSpace(decision.Reason) == "" {
			return Allowlist{}, fmt.Errorf("%s: %s/%s %q has no reason", path, decision.Section, decision.Kind, decision.Value)
		}
	}
	return allow, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
