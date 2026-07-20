package releaseworkflow

const (
	DispatchInputUnit       = "unit"
	DispatchInputVersion    = "version"
	DispatchInputTag        = "tag"
	DispatchInputReleaseSHA = "release_sha"
)

type DispatchInputDefinition struct {
	Name        string
	Description string
}

func CanonicalDispatchInputContract() []DispatchInputDefinition {
	return []DispatchInputDefinition{
		{Name: DispatchInputUnit, Description: "Neko Release V2 unit id"},
		{Name: DispatchInputVersion, Description: "Neko-authoritative release version"},
		{Name: DispatchInputTag, Description: "Neko-created unit tag"},
		{Name: DispatchInputReleaseSHA, Description: "Neko-created release commit SHA"},
	}
}

func CanonicalDispatchInputValues(unit, version, tag, releaseSHA string) map[string]string {
	return map[string]string{
		DispatchInputUnit:       unit,
		DispatchInputVersion:    version,
		DispatchInputTag:        tag,
		DispatchInputReleaseSHA: releaseSHA,
	}
}

// CanonicalDispatchInputs filters arbitrary values to the exact external
// workflow_dispatch input contract.
func CanonicalDispatchInputs(values map[string]string) map[string]string {
	contract := CanonicalDispatchInputContract()
	inputs := make(map[string]string, len(contract))
	for _, definition := range contract {
		inputs[definition.Name] = values[definition.Name]
	}
	return inputs
}
