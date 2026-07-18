package release

const (
	workflowDispatchInputUnit       = "unit"
	workflowDispatchInputVersion    = "version"
	workflowDispatchInputTag        = "tag"
	workflowDispatchInputReleaseSHA = "release_sha"
)

type workflowDispatchInputDefinition struct {
	Name        string
	Description string
}

func canonicalWorkflowDispatchInputContract() []workflowDispatchInputDefinition {
	return []workflowDispatchInputDefinition{
		{Name: workflowDispatchInputUnit, Description: "Neko Release V2 unit id"},
		{Name: workflowDispatchInputVersion, Description: "Neko-authoritative release version"},
		{Name: workflowDispatchInputTag, Description: "Neko-created unit tag"},
		{Name: workflowDispatchInputReleaseSHA, Description: "Neko-created release commit SHA"},
	}
}

func canonicalWorkflowDispatchInputValues(unit, version, tag, releaseSHA string) map[string]string {
	return map[string]string{
		workflowDispatchInputUnit:       unit,
		workflowDispatchInputVersion:    version,
		workflowDispatchInputTag:        tag,
		workflowDispatchInputReleaseSHA: releaseSHA,
	}
}
