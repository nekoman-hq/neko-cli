package release

import (
	"context"
	"fmt"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type integrationDoctorRemoteArtifactContract struct {
	UnitID         string
	Tag            string
	Reference      string
	RequiredAssets []string
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectInstallationArtifacts(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	request integrationDoctorRemoteRequest,
	inspection *integrationDoctorRemoteInspection,
) {
	contracts := integrationDoctorInstallationRemoteContracts(request, remote.variableValues)
	if len(contracts) == 0 {
		inspection.append(integrationDoctorRemoteFact(
			"installation-artifacts", "installation_wiring", integrationDoctorNotAttempted,
			"Exact installation release identities were not attempted because recognized pinned versions or local artifact contracts were unavailable.",
			"", "", "install.sh",
		), nil)
		return
	}
	for _, contract := range contracts {
		observation := inspector.observeRelease(ctx, remote, contract.Tag)
		fact := integrationDoctorRemoteFact(
			contract.Tag, "installation_wiring", observation.Outcome.State,
			integrationDoctorRemoteOutcomeEvidence("Exact installation release "+contract.Tag, observation.Outcome),
			"", contract.UnitID, contract.Reference,
		)
		var diagnostic *integrationDoctorDiagnostic
		if observation.Outcome.State == integrationDoctorVerified {
			missing := integrationDoctorMissingRemoteAssets(observation.Release.Assets, contract.RequiredAssets)
			switch {
			case observation.Release.Draft || observation.Release.Prerelease:
				fact.State = integrationDoctorMismatch
				fact.Evidence = "The exact installation release exists but is a draft or prerelease."
				diagnostic = integrationDoctorRemoteDiagnostic(
					integrationDoctorError, "REMOTE_INSTALLATION_RELEASE_INVALID", "", contract.UnitID,
					fmt.Sprintf("Exact installation release %q is not a stable published release.", contract.Tag),
					"Publish the exact pinned installation identity as a non-draft stable release.",
				)
			case len(missing) > 0:
				fact.State = integrationDoctorMismatch
				fact.Evidence = "The exact installation release is missing required exact assets: " + strings.Join(missing, ", ") + "."
				diagnostic = integrationDoctorRemoteDiagnostic(
					integrationDoctorError, "REMOTE_INSTALLATION_ASSET_MISSING", "", contract.UnitID,
					fmt.Sprintf("Exact installation release %q is missing required assets: %s.", contract.Tag, strings.Join(missing, ", ")),
					"Publish every exact archive and checksum identity derived from the locally verified installer and GoReleaser contracts.",
				)
			default:
				fact.Evidence = fmt.Sprintf("Exact installation release %s contains all %d required archive and checksum identities.", contract.Tag, len(contract.RequiredAssets))
			}
		} else {
			diagnostic = integrationDoctorDiagnosticForRemoteOutcome(
				observation.Outcome, "REMOTE_INSTALLATION_RELEASE", "", contract.UnitID, "installation release "+contract.Tag,
			)
		}
		inspection.append(fact, diagnostic)
	}
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectPublicationTargets(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	request integrationDoctorRemoteRequest,
	inspection *integrationDoctorRemoteInspection,
) {
	units := integrationDoctorRemoteWorkflowUnits(request.Workflows)
	for _, unit := range units {
		contract, ok := integrationDoctorPublicationRemoteContract(request, unit)
		if !ok {
			inspection.append(integrationDoctorRemoteFact(
				unit.ID, "publication_identity", integrationDoctorUnsupported,
				"The local GoReleaser artifact contract was not supported for exact remote publication-state verification.",
				unit.Workflow, unit.ID, unit.Workflow,
			), integrationDoctorRemoteDiagnostic(
				integrationDoctorNotVerifiable, "REMOTE_PUBLICATION_ARTIFACTS_UNSUPPORTED", unit.Workflow, unit.ID,
				"The exact publication artifact identities could not be derived from the supported local contract.",
				"Use one supported focused GoReleaser configuration for the release unit.",
			))
			continue
		}
		inspector.inspectPublishedRelease(ctx, remote, unit, contract, inspection)
	}
	if integrationDoctorRemoteUnitsContainPlugin(units) {
		inspector.inspectPluginRegistryTarget(ctx, remote, request, inspection)
	}
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectPublishedRelease(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	unit releaseconfig.ReleaseUnit,
	contract integrationDoctorRemoteArtifactContract,
	inspection *integrationDoctorRemoteInspection,
) {
	releaseObservation := inspector.observeRelease(ctx, remote, contract.Tag)
	releaseFact := integrationDoctorRemoteFact(
		contract.Tag+"#release", "publication_identity", releaseObservation.Outcome.State,
		integrationDoctorRemoteOutcomeEvidence("Exact publication release "+contract.Tag, releaseObservation.Outcome),
		unit.Workflow, unit.ID, contract.Reference,
	)
	var releaseDiagnostic *integrationDoctorDiagnostic
	if releaseObservation.Outcome.State == integrationDoctorVerified {
		missing := integrationDoctorMissingRemoteAssets(releaseObservation.Release.Assets, contract.RequiredAssets)
		if len(missing) > 0 {
			releaseFact.State = integrationDoctorMismatch
			releaseFact.Evidence = "The exact existing publication release is missing locally derived artifact identities: " + strings.Join(missing, ", ") + "."
			releaseDiagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorError, "REMOTE_PUBLICATION_ASSET_MISSING", unit.Workflow, unit.ID,
				fmt.Sprintf("Existing exact release %q is missing required artifact identities: %s.", contract.Tag, strings.Join(missing, ", ")),
				"Restore the exact artifacts for the already recorded release identity; future publication acceptance remains a separate uncertainty.",
			)
		} else {
			releaseFact.Evidence = fmt.Sprintf("Existing exact release %s contains all %d locally derived publication artifacts.", contract.Tag, len(contract.RequiredAssets))
		}
	} else {
		releaseDiagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			releaseObservation.Outcome, "REMOTE_PUBLICATION_RELEASE", unit.Workflow, unit.ID, "existing publication release "+contract.Tag,
		)
	}
	inspection.append(releaseFact, releaseDiagnostic)

	tagObservation := inspector.observeTag(ctx, remote, contract.Tag)
	tagFact := integrationDoctorRemoteFact(
		contract.Tag+"#tag", "publication_identity", tagObservation.Outcome.State,
		integrationDoctorRemoteOutcomeEvidence("Exact publication tag "+contract.Tag, tagObservation.Outcome),
		unit.Workflow, unit.ID, contract.Reference,
	)
	var tagDiagnostic *integrationDoctorDiagnostic
	if tagObservation.Outcome.State == integrationDoctorVerified {
		tagFact.Evidence = fmt.Sprintf("Exact remote tag %s exists and resolves to a %s object.", contract.Tag, tagObservation.Reference.ObjectType)
	} else {
		tagDiagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			tagObservation.Outcome, "REMOTE_PUBLICATION_TAG", unit.Workflow, unit.ID, "existing publication tag "+contract.Tag,
		)
	}
	inspection.append(tagFact, tagDiagnostic)
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectPluginRegistryTarget(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	request integrationDoctorRemoteRequest,
	inspection *integrationDoctorRemoteInspection,
) {
	const tag = "plugin-registry"
	observation := inspector.observeRelease(ctx, remote, tag)
	fact := integrationDoctorRemoteFact(
		tag, "publication_identity", observation.Outcome.State,
		integrationDoctorRemoteOutcomeEvidence("Exact plugin registry release", observation.Outcome),
		"", "", ".github/scripts/publish-plugin-index.sh",
	)
	var diagnostic *integrationDoctorDiagnostic
	if observation.Outcome.State == integrationDoctorVerified {
		missing := integrationDoctorMissingRemoteAssets(observation.Release.Assets, []string{"plugin-index.json"})
		if len(missing) > 0 {
			fact.State = integrationDoctorMismatch
			fact.Evidence = "The exact plugin-registry release is missing plugin-index.json."
			diagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorError, "REMOTE_PLUGIN_REGISTRY_ASSET_MISSING", "", "",
				"The exact plugin-registry release does not contain plugin-index.json.",
				"Restore the exact mutable registry asset without using latest-release discovery.",
			)
		} else {
			fact.Evidence = "The exact plugin-registry release contains plugin-index.json."
		}
	} else {
		diagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			observation.Outcome, "REMOTE_PLUGIN_REGISTRY_RELEASE", "", "", "plugin-registry release",
		)
	}
	inspection.append(fact, diagnostic)
	_ = request
}

func (inspector integrationDoctorGitHubRemoteInspector) observeRelease(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	tag string,
) integrationDoctorRemoteReleaseObservation {
	if observation, exists := remote.releases[tag]; exists {
		return observation
	}
	release, outcome := inspector.reader.ReleaseByTag(ctx, remote.identity, tag, remote.publicToken)
	observation := integrationDoctorRemoteReleaseObservation{Release: release, Outcome: outcome}
	remote.releases[tag] = observation
	return observation
}

func (inspector integrationDoctorGitHubRemoteInspector) observeTag(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	tag string,
) integrationDoctorRemoteTagObservation {
	if observation, exists := remote.tags[tag]; exists {
		return observation
	}
	reference, outcome := inspector.reader.TagReference(ctx, remote.identity, tag, remote.publicToken)
	observation := integrationDoctorRemoteTagObservation{Reference: reference, Outcome: outcome}
	remote.tags[tag] = observation
	return observation
}

func integrationDoctorInstallationRemoteContracts(
	request integrationDoctorRemoteRequest,
	variables map[string]string,
) []integrationDoctorRemoteArtifactContract {
	if request.Repository == nil || request.Files == nil {
		return nil
	}
	contracts := make([]integrationDoctorRemoteArtifactContract, 0, 2)
	if version := variables["NEKO_VERSION"]; version != "" {
		installer, err := request.Files.ReadFile(request.RepositoryRoot, "install.sh")
		prefix := integrationDoctorCLIInstallerArchivePrefix(installer)
		unit, _, configPath, ok := integrationDoctorFindCLIArtifactConfig(
			request.RepositoryRoot, request.Repository.Units, prefix, request.Files,
		)
		if err == nil && ok {
			contracts = append(contracts, integrationDoctorRemoteArtifactContract{
				UnitID: unit.ID, Tag: unit.TagPrefix + version, Reference: configPath,
				RequiredAssets: integrationDoctorCLIInstallerAssets(prefix),
			})
		}
	}
	if version := variables["NEKO_RELEASE_PLUGIN_VERSION"]; version != "" {
		unit, present := integrationDoctorReleasePluginUnit(request.Repository.Units)
		if present {
			_, configPath, ok := integrationDoctorLoadUnitGoReleaserConfig(request.RepositoryRoot, unit, request.Files)
			if ok {
				contracts = append(contracts, integrationDoctorRemoteArtifactContract{
					UnitID: unit.ID, Tag: unit.TagPrefix + version, Reference: configPath,
					RequiredAssets: integrationDoctorPluginInstallerAssets(unit.PluginAssetPrefix, version),
				})
			}
		}
	}
	sort.Slice(contracts, func(left, right int) bool { return contracts[left].Tag < contracts[right].Tag })
	return contracts
}

func integrationDoctorPublicationRemoteContract(
	request integrationDoctorRemoteRequest,
	unit releaseconfig.ReleaseUnit,
) (integrationDoctorRemoteArtifactContract, bool) {
	if request.Files == nil {
		return integrationDoctorRemoteArtifactContract{}, false
	}
	config, configPath, ok := integrationDoctorLoadUnitGoReleaserConfig(request.RepositoryRoot, unit, request.Files)
	if !ok {
		return integrationDoctorRemoteArtifactContract{}, false
	}
	prefix := config.ProjectName
	if unit.IsPlugin {
		prefix = unit.PluginAssetPrefix
	}
	assets := integrationDoctorGoReleaserPublicationAssets(config, prefix, unit.Version, unit.IsPlugin)
	if len(assets) == 0 {
		return integrationDoctorRemoteArtifactContract{}, false
	}
	return integrationDoctorRemoteArtifactContract{
		UnitID: unit.ID, Tag: unit.TagPrefix + unit.Version, Reference: configPath, RequiredAssets: assets,
	}, true
}

func integrationDoctorCLIInstallerAssets(prefix string) []string {
	return integrationDoctorPlatformArchiveAssets(prefix, "", map[string]string{
		"Darwin": "tar.gz", "Linux": "tar.gz", "Windows": "zip",
	})
}

func integrationDoctorPluginInstallerAssets(prefix, version string) []string {
	assets := integrationDoctorPlatformArchiveAssets(prefix, version, map[string]string{
		"Darwin": "tar.gz", "Linux": "tar.gz", "Windows": "tar.gz",
	})
	assets = append(assets, prefix+"_"+version+"_checksums.txt")
	sort.Strings(assets)
	return assets
}

func integrationDoctorGoReleaserPublicationAssets(
	config integrationDoctorGoReleaserConfig,
	prefix string,
	version string,
	plugin bool,
) []string {
	formats := map[string]string{"Darwin": "tar.gz", "Linux": "tar.gz", "Windows": "tar.gz"}
	archive, present := integrationDoctorGoReleaserArchiveByID(config.Archives, prefix)
	if !present {
		return nil
	}
	for _, override := range archive.FormatOverrides {
		if len(override.Formats) == 0 {
			continue
		}
		switch override.Goos {
		case "darwin":
			formats["Darwin"] = override.Formats[0]
		case "linux":
			formats["Linux"] = override.Formats[0]
		case "windows":
			formats["Windows"] = override.Formats[0]
		}
	}
	nameVersion := ""
	if plugin {
		nameVersion = version
	}
	assets := integrationDoctorPlatformArchiveAssets(prefix, nameVersion, formats)
	if config.Checksum != nil {
		assets = append(assets, prefix+"_"+version+"_checksums.txt")
	}
	sort.Strings(assets)
	return assets
}

func integrationDoctorPlatformArchiveAssets(
	prefix string,
	version string,
	formats map[string]string,
) []string {
	assets := make([]string, 0, len(formats)*3)
	for _, operatingSystem := range []string{"Darwin", "Linux", "Windows"} {
		format := formats[operatingSystem]
		if format == "" {
			continue
		}
		for _, architecture := range []string{"arm64", "i386", "x86_64"} {
			parts := []string{prefix}
			if version != "" {
				parts = append(parts, version)
			}
			parts = append(parts, operatingSystem, architecture)
			assets = append(assets, strings.Join(parts, "_")+"."+format)
		}
	}
	sort.Strings(assets)
	return assets
}

func integrationDoctorMissingRemoteAssets(existing, required []string) []string {
	present := make(map[string]struct{}, len(existing))
	for _, asset := range existing {
		present[asset] = struct{}{}
	}
	missing := make([]string, 0)
	for _, asset := range required {
		if _, exists := present[asset]; !exists {
			missing = append(missing, asset)
		}
	}
	sort.Strings(missing)
	return missing
}

func integrationDoctorRemoteWorkflowUnits(
	workflows []integrationDoctorRemoteWorkflow,
) []releaseconfig.ReleaseUnit {
	unitsByID := make(map[string]releaseconfig.ReleaseUnit)
	for _, workflow := range workflows {
		for _, unit := range workflow.Units {
			unitsByID[unit.ID] = unit
		}
	}
	units := make([]releaseconfig.ReleaseUnit, 0, len(unitsByID))
	for _, unit := range unitsByID {
		units = append(units, unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return units
}

func integrationDoctorRemoteUnitsContainPlugin(units []releaseconfig.ReleaseUnit) bool {
	for _, unit := range units {
		if unit.IsPlugin {
			return true
		}
	}
	return false
}
