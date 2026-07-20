package config

// NormalizeV2Repository applies internal defaults without mutating files.
func NormalizeV2Repository(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState) *ReleaseRepository {
	units := make([]ReleaseUnit, 0, len(cfg.Units))
	for _, unit := range cfg.Units {
		workingDirectory := unit.WorkingDirectory
		if workingDirectory == "" {
			workingDirectory = "."
		}
		delivery := unit.Executor.Delivery
		units = append(units, ReleaseUnit{
			ID:               unit.ID,
			DisplayName:      unit.DisplayName,
			Paths:            append([]string(nil), unit.Paths...),
			WorkingDirectory: workingDirectory,
			TagPrefix:        unit.TagPrefix,
			Kind:             string(unit.Kind),
			ExecutorType:     string(unit.Executor.Type),
			Delivery:         string(delivery),
			Workflow:         unit.Executor.Workflow,
			Version:          state.Units[unit.ID].Version,
		})
		if unit.Plugin != nil {
			last := &units[len(units)-1]
			last.IsPlugin = true
			last.PluginName = unit.Plugin.Name
			last.PluginManifestPath = unit.Plugin.Manifest
			last.PluginAssetPrefix = unit.Plugin.AssetPrefix
			last.PluginBinaryName = unit.Plugin.BinaryName
		}
	}

	return &ReleaseRepository{
		RepositoryRoot: repositoryRoot,
		SchemaVersion:  2,
		SourceFormat:   SourceFormatV2,
		Units:          units,
	}
}
