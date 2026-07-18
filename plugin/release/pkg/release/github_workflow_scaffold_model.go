package release

type githubWorkflowScaffoldIntent string

const (
	githubWorkflowScaffoldCreateIntent  githubWorkflowScaffoldIntent = "create"
	githubWorkflowScaffoldPreviewIntent githubWorkflowScaffoldIntent = "preview"
)

type githubWorkflowTargetClassification string

const (
	githubWorkflowTargetCreate    githubWorkflowTargetClassification = "create"
	githubWorkflowTargetUnchanged githubWorkflowTargetClassification = "unchanged"
	githubWorkflowTargetConflict  githubWorkflowTargetClassification = "conflict"
)

type githubWorkflowScaffoldRequest struct {
	RepositoryRoot string
	UnitID         string
	TargetPath     string
}

type githubWorkflowScaffoldCommandRequest struct {
	Scaffold githubWorkflowScaffoldRequest
	Intent   githubWorkflowScaffoldIntent
}

type githubWorkflowOutputTarget struct {
	RepositoryRoot string
	RelativePath   string
	AbsolutePath   string
}

//nolint:govet // Fields follow the stable response order.
type githubWorkflowGenerationPlan struct {
	Target             githubWorkflowOutputTarget
	Classification     githubWorkflowTargetClassification
	SelectedUnit       string
	UnitsUsingWorkflow []string
	GeneratedContent   []byte
	ConflictReason     string
	ContractVersion    int
}

//nolint:govet // Fields follow the stable response order.
type githubWorkflowScaffoldResult struct {
	Plan      githubWorkflowGenerationPlan
	Action    string
	Guidance  string
	Written   bool
	Unchanged bool
	Preview   bool
}
