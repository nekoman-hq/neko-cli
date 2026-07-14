package init

type v2RepositoryPresence struct {
	LegacyConfig bool
	Config       bool
	State        bool
}

func evaluateV2InitializationPolicy(presence v2RepositoryPresence, force bool) *v2PresencePolicyFailure {
	hasAnyV2 := presence.Config || presence.State
	if presence.LegacyConfig && hasAnyV2 {
		return &v2PresencePolicyFailure{
			code:    "CONFIG_CONFLICT",
			message: "release configuration conflict: .release.neko.json and V2 files both exist. Resolve the conflict explicitly before running init.",
		}
	}
	if presence.LegacyConfig {
		return &v2PresencePolicyFailure{
			code:    "V1_CONFIG_EXISTS",
			message: ".release.neko.json already exists. Run 'neko release migrate' to convert it to V2; init will not overwrite V1 configs.",
		}
	}
	if hasAnyV2 && !force {
		return &v2PresencePolicyFailure{
			code:    "CONFIG_EXISTS",
			message: ".neko/release.config.json or .neko/release.state.json already exists. Use --force to recreate both V2 files.",
		}
	}
	return nil
}

func evaluateV2UnitAdditionPolicy(presence v2RepositoryPresence) *v2PresencePolicyFailure {
	if presence.LegacyConfig && (presence.Config || presence.State) {
		return &v2PresencePolicyFailure{
			code:    "CONFIG_CONFLICT",
			message: "release configuration conflict: .release.neko.json and V2 files both exist. Resolve the conflict explicitly before running unit-add.",
		}
	}
	if presence.LegacyConfig {
		return &v2PresencePolicyFailure{
			code:    "V1_CONFIG_EXISTS",
			message: ".release.neko.json exists. Run 'neko release migrate' before adding V2 units.",
		}
	}
	if !presence.Config && !presence.State {
		return &v2PresencePolicyFailure{
			code:    "V2_CONFIG_MISSING",
			message: ".neko/release.config.json and .neko/release.state.json are missing. Run 'neko release init' first.",
		}
	}
	if !presence.Config || !presence.State {
		return &v2PresencePolicyFailure{
			code:    "PARTIAL_V2_CONFIG",
			message: "partial V2 configuration: both .neko/release.config.json and .neko/release.state.json are required before unit-add.",
		}
	}
	return nil
}
