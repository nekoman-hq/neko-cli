package config

import (
	"fmt"
	"sort"
	"strings"
)

// UnitResolutionOptions controls command-specific unit selection behavior.
type UnitResolutionOptions struct {
	RequireExplicitForMulti bool
}

// ResolveReleaseUnit selects one unit from the canonical repository model.
func ResolveReleaseUnit(repository *ReleaseRepository, requestedUnit string, opts UnitResolutionOptions) (*ReleaseUnit, error) {
	if repository == nil {
		return nil, fmt.Errorf("release repository is missing")
	}
	if len(repository.Units) == 0 {
		return nil, fmt.Errorf("release repository contains no units")
	}

	requestedUnit = strings.TrimSpace(requestedUnit)
	if requestedUnit != "" {
		for i := range repository.Units {
			if repository.Units[i].ID == requestedUnit {
				return &repository.Units[i], nil
			}
		}
		return nil, fmt.Errorf("unknown release unit %q; available units: %s", requestedUnit, strings.Join(repository.UnitIDs(), ", "))
	}

	if len(repository.Units) == 1 {
		return &repository.Units[0], nil
	}
	if opts.RequireExplicitForMulti {
		return nil, fmt.Errorf("release unit is required for this command; available units: %s", strings.Join(repository.UnitIDs(), ", "))
	}

	return nil, fmt.Errorf("release unit is ambiguous; available units: %s", strings.Join(repository.UnitIDs(), ", "))
}

// UnitIDs returns stable sorted unit IDs for diagnostics.
func (repository *ReleaseRepository) UnitIDs() []string {
	ids := make([]string, 0, len(repository.Units))
	for _, unit := range repository.Units {
		ids = append(ids, unit.ID)
	}
	sort.Strings(ids)
	return ids
}
