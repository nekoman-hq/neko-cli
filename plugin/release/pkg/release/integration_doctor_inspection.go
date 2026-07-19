package release

import (
	"context"
	"fmt"
	"sort"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type integrationDoctorInspector interface {
	Inspect(context.Context, integrationDoctorRequest) *integrationDoctorResult
}

type integrationDoctorInspectionUseCase struct {
	sources    integrationDoctorSourceReader
	workflows  integrationDoctorWorkflowReader
	files      integrationDoctorRepositoryFileReader
	identities integrationDoctorRepositoryIdentityReader
}

func (useCase integrationDoctorInspectionUseCase) Inspect(
	_ context.Context,
	request integrationDoctorRequest,
) *integrationDoctorResult {
	result := &integrationDoctorResult{
		Units:         make([]integrationDoctorUnit, 0),
		Workflows:     make([]integrationDoctorWorkflow, 0),
		Verifications: make([]integrationDoctorVerification, 0),
		Diagnostics:   make([]integrationDoctorDiagnostic, 0),
	}
	source := inspectIntegrationDoctorSource(request.RepositoryRoot, useCase.sources.Read(request.RepositoryRoot))
	result.Diagnostics = append(result.Diagnostics, source.Diagnostics...)
	if source.Repository == nil {
		finalizeIntegrationDoctorResult(result)
		return result
	}

	selectedUnits, selectionDiagnostic := selectIntegrationDoctorUnits(source.Repository, request.UnitID)
	if selectionDiagnostic != nil {
		result.Diagnostics = append(result.Diagnostics, *selectionDiagnostic)
		finalizeIntegrationDoctorResult(result)
		return result
	}
	for _, unit := range selectedUnits {
		result.Units = append(result.Units, integrationDoctorUnit{
			ID: unit.ID, Version: unit.Version, TagPrefix: unit.TagPrefix,
			Executor: unit.ExecutorType, Delivery: unit.Delivery, Workflow: unit.Workflow,
			WorkingDirectory: unit.WorkingDirectory,
		})
	}

	workflowUnits := integrationDoctorWorkflowUnits(source.Repository, selectedUnits)
	repositoryIdentity, repositoryIdentityErr := useCase.identities.ReadOrigin(request.RepositoryRoot)
	paths := make([]string, 0, len(workflowUnits))
	for path := range workflowUnits {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fact, verifications, diagnostics := inspectIntegrationDoctorWorkflow(
			request.RepositoryRoot,
			path,
			workflowUnits[path],
			source.Repository.Units,
			useCase.workflows.Read(request.RepositoryRoot, path),
			useCase.files,
			repositoryIdentity,
			repositoryIdentityErr,
		)
		result.Workflows = append(result.Workflows, fact)
		result.Verifications = append(result.Verifications, verifications...)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	finalizeIntegrationDoctorResult(result)
	return result
}

func selectIntegrationDoctorUnits(
	repository *releaseconfig.ReleaseRepository,
	selectedUnit string,
) ([]releaseconfig.ReleaseUnit, *integrationDoctorDiagnostic) {
	if selectedUnit != "" {
		unit, err := releaseconfig.ResolveReleaseUnit(repository, selectedUnit, releaseconfig.UnitResolutionOptions{})
		if err != nil {
			diagnostic := newIntegrationDoctorDiagnostic(
				integrationDoctorError,
				"unit",
				"RELEASE_UNIT_NOT_FOUND",
				fmt.Sprintf("Release V2 unit %q is not configured.", selectedUnit),
				"Select an exact unit id from .neko/release.config.json.",
			)
			diagnostic.Unit = selectedUnit
			return nil, &diagnostic
		}
		return []releaseconfig.ReleaseUnit{*unit}, nil
	}
	units := append([]releaseconfig.ReleaseUnit(nil), repository.Units...)
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return units, nil
}

func integrationDoctorWorkflowUnits(
	repository *releaseconfig.ReleaseRepository,
	selected []releaseconfig.ReleaseUnit,
) map[string][]releaseconfig.ReleaseUnit {
	selectedPaths := map[string]struct{}{}
	for _, unit := range selected {
		selectedPaths[unit.Workflow] = struct{}{}
	}
	units := map[string][]releaseconfig.ReleaseUnit{}
	for _, unit := range repository.Units {
		if _, inspect := selectedPaths[unit.Workflow]; inspect {
			units[unit.Workflow] = append(units[unit.Workflow], unit)
		}
	}
	for path := range units {
		sort.Slice(units[path], func(left, right int) bool {
			return units[path][left].ID < units[path][right].ID
		})
	}
	return units
}

var _ integrationDoctorInspector = integrationDoctorInspectionUseCase{}
