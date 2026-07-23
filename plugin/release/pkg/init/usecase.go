package init

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type v2PresencePolicyFailure struct {
	code    string
	message string
}

func (failure *v2PresencePolicyFailure) Error() string {
	return failure.message
}

type commandFailure struct { //nolint:govet // Failure fields retain code/message/details presentation order.
	code    string
	message string
	details map[string]any
	origin  commandFailureOrigin
}

type commandFailureOrigin uint8

const commandFailureFromPresencePolicy commandFailureOrigin = 1

type initializeV2Result struct {
	Unit         v2InitConfig
	ConfigAction string
	StateAction  string
	Force        bool
}

type addV2UnitResult struct {
	Unit              v2InitConfig
	ExistingUnitCount int
}

type v2PresenceReader interface {
	Presence() v2RepositoryPresence
}

type v2PairLoader interface {
	LoadPair() (v2ReleasePair, error)
}

type v2PairValidator interface {
	ValidatePair(pair v2ReleasePair) error
}

type v2PairWriter interface {
	PersistPair(pair v2ReleasePair) error
}

type initializeV2RepositoryUseCase struct {
	presenceReader v2PresenceReader
	validator      v2PairValidator
	writer         v2PairWriter
}

func (useCase initializeV2RepositoryUseCase) Execute(request initCommandRequest) (initializeV2Result, *commandFailure) {
	log.PluginV(log.Init, "Inspecting repository initialization state")
	presence := useCase.presenceReader.Presence()
	if policyFailure := evaluateV2InitializationPolicy(presence, request.Force); policyFailure != nil {
		return initializeV2Result{}, mapPresencePolicyFailure(policyFailure)
	}
	if request.Force && (presence.Config || presence.State) {
		log.PluginV(log.Init, "Existing V2 artifacts accepted for replacement via --force")
	}

	log.PluginV(log.Init, "Resolving V2 release unit configuration")
	unit, err := constructV2Unit(request.Unit)
	if err != nil {
		return initializeV2Result{}, &commandFailure{
			code:    "INVALID_FLAGS",
			message: err.Error(),
			details: initInvalidFlagDetails(),
		}
	}
	pair := newV2ReleasePair(unit)
	log.PluginV(log.Init, "Validating generated V2 configuration and state")
	if err := useCase.validator.ValidatePair(pair); err != nil {
		return initializeV2Result{}, &commandFailure{code: "VALIDATION_ERROR", message: err.Error()}
	}
	log.PluginV(log.Init, "Preparing V2 configuration and state artifacts")
	log.PluginV(log.Init, "Writing V2 configuration and state")
	if err := useCase.writer.PersistPair(pair); err != nil {
		return initializeV2Result{}, &commandFailure{
			code:    "SAVE_ERROR",
			message: fmt.Sprintf("Failed to save V2 release configuration: %v", err),
		}
	}
	log.PluginV(log.Init, "V2 configuration and state write completed")
	return initializeV2Result{
		Unit: unit.Config, ConfigAction: setupArtifactAction(presence.Config),
		StateAction: setupArtifactAction(presence.State), Force: request.Force,
	}, nil
}

type addV2ReleaseUnitUseCase struct {
	presenceReader v2PresenceReader
	loader         v2PairLoader
	validator      v2PairValidator
	writer         v2PairWriter
}

func (useCase addV2ReleaseUnitUseCase) Execute(request unitAddCommandRequest) (addV2UnitResult, *commandFailure) {
	log.PluginV(log.Init, "Inspecting existing V2 configuration and state")
	if policyFailure := evaluateV2UnitAdditionPolicy(useCase.presenceReader.Presence()); policyFailure != nil {
		return addV2UnitResult{}, mapPresencePolicyFailure(policyFailure)
	}
	if strings.TrimSpace(request.Unit.UnitID) == "" {
		return addV2UnitResult{}, &commandFailure{code: "INVALID_FLAGS", message: "missing required flag: --unit"}
	}
	if request.ForcePresent {
		return addV2UnitResult{}, &commandFailure{
			code:    "INVALID_FLAGS",
			message: "unit-add does not support --force; existing units are never overwritten",
		}
	}

	log.PluginV(log.Init, "Resolving release unit defaults")
	unit, err := constructV2Unit(request.Unit)
	if err != nil {
		return addV2UnitResult{}, &commandFailure{
			code:    "INVALID_FLAGS",
			message: err.Error(),
			details: unitAddInvalidFlagDetails(),
		}
	}

	log.PluginV(log.Init, "Reading existing V2 configuration and state")
	current, err := useCase.loader.LoadPair()
	if err != nil {
		var loadError *v2PairLoadError
		if errors.As(err, &loadError) {
			if loadError.part == "state" {
				return addV2UnitResult{}, &commandFailure{
					code:    "LOAD_ERROR",
					message: fmt.Sprintf("Failed to load V2 release state: %v", loadError.err),
				}
			}
			return addV2UnitResult{}, &commandFailure{
				code:    "LOAD_ERROR",
				message: fmt.Sprintf("Failed to load V2 release config: %v", loadError.err),
			}
		}
		return addV2UnitResult{}, &commandFailure{code: "LOAD_ERROR", message: err.Error()}
	}
	if current.State.Units == nil {
		return addV2UnitResult{}, &commandFailure{code: "VALIDATION_ERROR", message: "v2 state units must be present"}
	}
	log.PluginV(log.Init, "Checking duplicate unit identity")
	if _, exists := current.State.Units[unit.Unit.ID]; exists {
		return addV2UnitResult{}, &commandFailure{
			code:    "DUPLICATE_UNIT",
			message: fmt.Sprintf("release unit %q already exists in state", unit.Unit.ID),
		}
	}

	updated := appendV2ReleaseUnit(current, unit)
	log.PluginV(log.Init, "Validating updated V2 configuration and state")
	if err := useCase.validator.ValidatePair(updated); err != nil {
		return addV2UnitResult{}, &commandFailure{code: "VALIDATION_ERROR", message: err.Error()}
	}
	log.PluginV(log.Init, "Writing updated V2 configuration and state")
	if err := useCase.writer.PersistPair(updated); err != nil {
		return addV2UnitResult{}, &commandFailure{
			code:    "SAVE_ERROR",
			message: fmt.Sprintf("Failed to save V2 release configuration: %v", err),
		}
	}
	log.PluginV(log.Init, "Release unit append completed")
	return addV2UnitResult{Unit: unit.Config, ExistingUnitCount: len(current.Config.Units)}, nil
}

func setupArtifactAction(exists bool) string {
	if exists {
		return "replaced"
	}
	return "created"
}

func mapPresencePolicyFailure(policyFailure *v2PresencePolicyFailure) *commandFailure {
	return &commandFailure{
		code:    policyFailure.code,
		message: policyFailure.message,
		origin:  commandFailureFromPresencePolicy,
	}
}

func initInvalidFlagDetails() map[string]any {
	return map[string]any{
		"required_flags": []string{"executor", "delivery", "workflow"},
		"optional_flags": []string{
			"unit",
			"display-name",
			"version",
			"workflow",
			"tag-prefix",
			"working-directory",
			"paths",
			"kind",
			"plugin-name",
			"plugin-manifest",
			"plugin-asset-prefix",
			"plugin-binary-name",
			"force",
		},
	}
}

func unitAddInvalidFlagDetails() map[string]any {
	return map[string]any{
		"required_flags": []string{"unit", "executor", "delivery", "workflow"},
		"optional_flags": []string{
			"display-name",
			"version",
			"workflow",
			"tag-prefix",
			"working-directory",
			"paths",
			"kind",
			"plugin-name",
			"plugin-manifest",
			"plugin-asset-prefix",
			"plugin-binary-name",
		},
	}
}
