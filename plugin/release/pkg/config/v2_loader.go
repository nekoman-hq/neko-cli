package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadV2Repository strictly loads, validates, and normalizes a V2 repository.
func LoadV2Repository(repositoryRoot string) (*ReleaseRepository, error) {
	cfg, err := LoadV2Config(V2ConfigPath(repositoryRoot))
	if err != nil {
		return nil, err
	}
	state, err := LoadV2State(V2StatePath(repositoryRoot))
	if err != nil {
		return nil, err
	}
	if err := ValidateV2(repositoryRoot, cfg, state); err != nil {
		return nil, err
	}
	return NormalizeV2Repository(repositoryRoot, cfg, state), nil
}

// LoadV2Config strictly decodes a V2 release config file.
func LoadV2Config(path string) (*V2ReleaseConfig, error) {
	var cfg V2ReleaseConfig
	if err := readStrictJSON(path, &cfg); err != nil {
		return nil, fmt.Errorf("v2 config %s: %w", path, err)
	}
	return &cfg, nil
}

// LoadV2State strictly decodes a V2 release state file.
func LoadV2State(path string) (*V2ReleaseState, error) {
	var state V2ReleaseState
	if err := readStrictJSON(path, &state); err != nil {
		return nil, fmt.Errorf("v2 state %s: %w", path, err)
	}
	return &state, nil
}

func readStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON content")
	}
	return nil
}
