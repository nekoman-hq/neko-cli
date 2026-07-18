package init

import (
	"errors"
	"fmt"
	"strings"
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
	Unit v2InitConfig
}

type addV2UnitResult struct {
	Unit v2InitConfig
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
	if policyFailure := evaluateV2InitializationPolicy(useCase.presenceReader.Presence(), request.Force); policyFailure != nil {
		return initializeV2Result{}, mapPresencePolicyFailure(policyFailure)
	}

	unit, err := constructV2Unit(request.Unit)
	if err != nil {
		return initializeV2Result{}, &commandFailure{
			code:    "INVALID_FLAGS",
			message: err.Error(),
			details: initInvalidFlagDetails(),
		}
	}
	pair := newV2ReleasePair(unit)
	if err := useCase.validator.ValidatePair(pair); err != nil {
		return initializeV2Result{}, &commandFailure{code: "VALIDATION_ERROR", message: err.Error()}
	}
	if err := useCase.writer.PersistPair(pair); err != nil {
		return initializeV2Result{}, &commandFailure{
			code:    "SAVE_ERROR",
			message: fmt.Sprintf("Failed to save V2 release configuration: %v", err),
		}
	}
	return initializeV2Result{Unit: unit.Config}, nil
}

type addV2ReleaseUnitUseCase struct {
	presenceReader v2PresenceReader
	loader         v2PairLoader
	validator      v2PairValidator
	writer         v2PairWriter
}

func (useCase addV2ReleaseUnitUseCase) Execute(request unitAddCommandRequest) (addV2UnitResult, *commandFailure) {
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

	unit, err := constructV2Unit(request.Unit)
	if err != nil {
		return addV2UnitResult{}, &commandFailure{
			code:    "INVALID_FLAGS",
			message: err.Error(),
			details: unitAddInvalidFlagDetails(),
		}
	}

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
	if _, exists := current.State.Units[unit.Unit.ID]; exists {
		return addV2UnitResult{}, &commandFailure{
			code:    "DUPLICATE_UNIT",
			message: fmt.Sprintf("release unit %q already exists in state", unit.Unit.ID),
		}
	}

	updated := appendV2ReleaseUnit(current, unit)
	if err := useCase.validator.ValidatePair(updated); err != nil {
		return addV2UnitResult{}, &commandFailure{code: "VALIDATION_ERROR", message: err.Error()}
	}
	if err := useCase.writer.PersistPair(updated); err != nil {
		return addV2UnitResult{}, &commandFailure{
			code:    "SAVE_ERROR",
			message: fmt.Sprintf("Failed to save V2 release configuration: %v", err),
		}
	}
	return addV2UnitResult{Unit: unit.Config}, nil
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
