package pipelineinspection

const pipelineCommandName = "pipeline"

type pipelineStatus string

const (
	pipelineReady   pipelineStatus = "ready"
	pipelineActive  pipelineStatus = "active"
	pipelineInvalid pipelineStatus = "invalid"
)

type pipelineRequest struct {
	RepositoryRoot string
	UnitID         string
}

// StageOwner identifies the component that owns a configured operation.
type StageOwner string

// StageLocation identifies where a configured operation executes.
type StageLocation string

// MutationClass identifies the strongest mutation category an operation may
// perform during a real release execution.
type MutationClass string

// ConfigurationStatus describes static configuration presence, not runtime
// completion or journal progress.
type ConfigurationStatus string

// RuntimeStatus describes locally observed lifecycle evidence. It never
// drives an execution transition.
type RuntimeStatus string

const (
	StageOwnerNekoCLI          StageOwner = "Neko CLI"
	StageOwnerLocalGit         StageOwner = "local Git"
	StageOwnerRemoteGit        StageOwner = "remote Git"
	StageOwnerGitHubAPI        StageOwner = "GitHub API"
	StageOwnerConsumerWorkflow StageOwner = "consumer workflow"
	StageOwnerReleaseTool      StageOwner = "release tool"
)

const (
	StageLocationLocalProcess        StageLocation = "local process"
	StageLocationLocalRepository     StageLocation = "local repository"
	StageLocationLocalGit            StageLocation = "local Git"
	StageLocationRemoteGit           StageLocation = "remote Git"
	StageLocationGitHubAPI           StageLocation = "GitHub API"
	StageLocationGitHubActionsRunner StageLocation = "GitHub Actions runner"
)

const (
	MutationNone         MutationClass = "none"
	MutationFilesystem   MutationClass = "filesystem"
	MutationReleaseState MutationClass = "release state"
	MutationGitIndex     MutationClass = "Git index"
	MutationGitObject    MutationClass = "Git object"
	MutationGitRef       MutationClass = "Git ref"
	MutationRemoteGit    MutationClass = "remote Git"
	MutationRemoteAPI    MutationClass = "remote API"
	MutationPublication  MutationClass = "publication"
)

const StageConfigured ConfigurationStatus = "configured"

const (
	RuntimeNotStarted  RuntimeStatus = "not_started"
	RuntimePending     RuntimeStatus = "pending"
	RuntimeConfirmed   RuntimeStatus = "confirmed"
	RuntimeInvalid     RuntimeStatus = "invalid"
	RuntimeNotObserved RuntimeStatus = "not_observed"
)

// LifecycleStage is immutable descriptive metadata supplied by the
// authoritative root release coordinator. It contains no executable behavior.
//
//nolint:govet // Field order follows the stable machine contract.
type LifecycleStage struct {
	ID                  string              `json:"id"`
	Label               string              `json:"label"`
	Owner               StageOwner          `json:"owner"`
	Location            StageLocation       `json:"location"`
	Mutation            MutationClass       `json:"mutation"`
	ConfigurationStatus ConfigurationStatus `json:"configuration_status"`
	Source              string              `json:"source"`
	ConditionalReason   string              `json:"conditional_reason,omitempty"`
	RuntimeStatus       RuntimeStatus       `json:"runtime_status"`
	RuntimeEvidence     string              `json:"runtime_evidence,omitempty"`
	RuntimeReason       string              `json:"runtime_reason,omitempty"`
	RuntimeIdentity     string              `json:"runtime_identity,omitempty"`
	RuntimeConfirmedAt  string              `json:"runtime_confirmed_at,omitempty"`
}

//nolint:govet // Field order follows the stable machine contract.
type pipelineUnit struct {
	ID                string `json:"id"`
	DisplayName       string `json:"display_name,omitempty"`
	Kind              string `json:"kind"`
	Executor          string `json:"executor"`
	Delivery          string `json:"delivery"`
	ConfiguredVersion string `json:"configured_version"`
	WorkingDirectory  string `json:"working_directory"`
}

type pipelineRelease struct {
	ConfiguredVersion string                     `json:"configured_version"`
	TagPrefix         string                     `json:"tag_prefix"`
	ConfiguredTag     string                     `json:"configured_tag"`
	MaterializedFiles []pipelineMaterializedFile `json:"materialized_files"`
}

type pipelineMaterializedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type pipelineRepository struct {
	SourceGeneration string `json:"source_generation"`
	LocalBranch      string `json:"local_branch"`
	LocalHead        string `json:"local_head"`
	Tracking         string `json:"tracking"`
}

//nolint:govet // Field order follows the stable machine contract.
type pipelineWorkflow struct {
	Path               string   `json:"path"`
	Delivery           string   `json:"delivery"`
	RequiredInputs     []string `json:"required_inputs"`
	ReleaseTool        string   `json:"release_tool"`
	ConsumerOperations []string `json:"consumer_operations"`
	Publication        string   `json:"publication"`
	PluginRegistry     string   `json:"plugin_registry"`
}

type pipelineProgressInspection struct {
	ExecutionProgress          string `json:"execution_progress"`
	JournalsInspected          bool   `json:"journals_inspected"`
	ResumeEligibilityEvaluated bool   `json:"resume_eligibility_evaluated"`
	RemoteStateInspected       bool   `json:"remote_state_inspected"`
}

// RuntimeSnapshot is an immutable read-only input assembled by pkg/release.
// It contains no stores, clients, callbacks, or mutation behavior.
type RuntimeSnapshot struct {
	Inspected        bool
	RepositoryRemote string
	Executions       []RuntimeExecutionObservation
	Problems         []RuntimeProblem
}

// RuntimeExecutionObservation is the safe projection of one local execution
// journal. Reference is repository-independent and never an absolute path.
//
//nolint:govet // Fields follow inspection and identity order.
type RuntimeExecutionObservation struct {
	Reference               string
	Identity                string
	RepositoryRemote        string
	UnitID                  string
	CurrentVersion          string
	NextVersion             string
	Tag                     string
	Executor                string
	Delivery                string
	WorkflowPath            string
	State                   string
	PendingAction           string
	ReleaseCommitSHA        string
	DispatchJournalIdentity string
	CreatedAt               string
	UpdatedAt               string
	Valid                   bool
	Unresolved              bool
	ConfirmedStageIDs       []string
	CurrentStageIDs         []string
	PendingStageID          string
	Problem                 string
}

// RuntimeProblem is a sanitized local inspection problem.
type RuntimeProblem struct {
	Kind      string
	UnitID    string
	Reference string
	Reason    string
}

type pipelineExecution struct {
	Present          bool                       `json:"present"`
	Identity         string                     `json:"identity"`
	JournalCount     int                        `json:"journal_count"`
	UnresolvedCount  int                        `json:"unresolved_count"`
	Validity         string                     `json:"validity"`
	State            string                     `json:"state"`
	PendingAction    string                     `json:"pending_action"`
	Terminal         bool                       `json:"terminal"`
	CreatedAt        string                     `json:"created_at,omitempty"`
	UpdatedAt        string                     `json:"updated_at,omitempty"`
	JournalReference string                     `json:"journal_reference,omitempty"`
	Observations     []pipelineExecutionJournal `json:"observations"`
}

type pipelineExecutionJournal struct {
	Identity   string `json:"identity"`
	Reference  string `json:"reference"`
	State      string `json:"state,omitempty"`
	Unresolved bool   `json:"unresolved"`
	Valid      bool   `json:"valid"`
	Problem    string `json:"problem,omitempty"`
}

//nolint:govet // Field order follows the stable schema-version-one contract.
type pipelineResult struct {
	SchemaVersion      int                        `json:"schema_version"`
	Status             pipelineStatus             `json:"status"`
	Unit               pipelineUnit               `json:"unit"`
	Release            pipelineRelease            `json:"release"`
	Repository         pipelineRepository         `json:"repository"`
	Workflow           pipelineWorkflow           `json:"workflow"`
	Stages             []LifecycleStage           `json:"stages"`
	ProgressInspection pipelineProgressInspection `json:"progress_inspection"`
	Execution          pipelineExecution          `json:"execution"`
	Limitations        []string                   `json:"limitations"`
	InvalidEvidence    bool                       `json:"-"`
}

type commandFailure struct {
	Code    string
	Message string
}
