package config

import "encoding/json"

// CanonicalV2Config returns the normalized JSON bytes future writers should use.
func CanonicalV2Config(cfg V2ReleaseConfig) ([]byte, error) {
	for index := range cfg.Units {
		if cfg.Units[index].WorkingDirectory == "" {
			cfg.Units[index].WorkingDirectory = "."
		}
	}
	return marshalCanonicalJSON(cfg)
}

// CanonicalV2State returns the normalized JSON bytes future writers should use.
func CanonicalV2State(state V2ReleaseState) ([]byte, error) {
	return marshalCanonicalJSON(state)
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
