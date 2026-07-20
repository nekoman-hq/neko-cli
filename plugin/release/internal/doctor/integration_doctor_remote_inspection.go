package doctor

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type integrationDoctorRemoteInspector interface {
	Inspect(context.Context, integrationDoctorRemoteRequest) integrationDoctorRemoteInspection
}

type integrationDoctorRemoteWorkflow struct {
	Path     string
	Units    []releaseconfig.ReleaseUnit
	Snapshot integrationDoctorWorkflowSnapshot
}

//nolint:govet // Field order follows local-to-remote evidence flow.
type integrationDoctorRemoteRequest struct {
	RepositoryRoot string
	Repository     *releaseconfig.ReleaseRepository
	Workflows      []integrationDoctorRemoteWorkflow
	Identity       integrationDoctorRepositoryIdentity
	IdentityErr    error
	Files          integrationDoctorRepositoryFileReader
}

//nolint:govet // Field order follows the additive result contract.
type integrationDoctorRemoteInspection struct {
	Summary       integrationDoctorRemoteSummary
	Verifications []integrationDoctorVerification
	Diagnostics   []integrationDoctorDiagnostic
}

type integrationDoctorGitHubRemoteInspector struct {
	reader integrationDoctorGitHubReader
	tokens githubReadTokenResolver
}

type integrationDoctorRemoteTokenAccess struct {
	resolver  githubReadTokenResolver
	token     githubReadToken
	attempted bool
	available bool
}

func (access *integrationDoctorRemoteTokenAccess) Resolve(
	ctx context.Context,
) (githubReadToken, bool) {
	if access.attempted {
		return access.token, access.available
	}
	access.attempted = true
	if access.resolver == nil {
		return githubReadToken{}, false
	}
	token, err := access.resolver.ResolveGitHubReadToken(ctx)
	if err != nil || token.secretValue() == "" {
		return githubReadToken{}, false
	}
	access.token = token
	access.available = true
	return access.token, true
}

//nolint:govet // Fields are grouped by remote observation concern.
type integrationDoctorRemoteContext struct {
	identity       integrationDoctorRepositoryIdentity
	repository     integrationDoctorGitHubRepository
	publicToken    githubReadToken
	protected      *integrationDoctorRemoteTokenAccess
	variableValues map[string]string
	releases       map[string]integrationDoctorRemoteReleaseObservation
	tags           map[string]integrationDoctorRemoteTagObservation
}

//nolint:govet // Logical result order keeps the decoded value before classification.
type integrationDoctorRemoteReleaseObservation struct {
	Release integrationDoctorGitHubRelease
	Outcome integrationDoctorGitHubReadOutcome
}

type integrationDoctorRemoteTagObservation struct {
	Reference integrationDoctorGitHubTagReference
	Outcome   integrationDoctorGitHubReadOutcome
}

func (inspector integrationDoctorGitHubRemoteInspector) Inspect(
	ctx context.Context,
	request integrationDoctorRemoteRequest,
) integrationDoctorRemoteInspection {
	inspection := integrationDoctorRemoteInspection{
		Summary: integrationDoctorRemoteSummary{
			Requested: true,
			Status:    integrationDoctorRemoteUnavailable,
		},
		Verifications: make([]integrationDoctorVerification, 0),
		Diagnostics:   make([]integrationDoctorDiagnostic, 0),
	}
	if inspector.reader == nil || request.IdentityErr != nil || request.Identity.Name() == "" {
		inspection.append(
			integrationDoctorRemoteFact(
				"repository", "remote_workflow_identity", integrationDoctorUnavailable,
				"The local origin identity could not be resolved for explicit remote verification.", "", "", ".git/config",
			),
			integrationDoctorRemoteDiagnostic(
				integrationDoctorNotVerifiable, "REMOTE_REPOSITORY_UNAVAILABLE", "", "",
				"The GitHub repository identity could not be resolved from the local origin.",
				"Configure one supported GitHub.com origin before requesting remote verification.",
			),
		)
		inspection.finalize()
		return inspection
	}

	tokens := &integrationDoctorRemoteTokenAccess{resolver: inspector.tokens}
	repository, repositoryOutcome, repositoryToken := inspector.inspectRepository(
		ctx, request.Identity, tokens,
	)
	repositoryFact, repositoryDiagnostic := integrationDoctorMapRemoteRepository(
		request.Identity, repository, repositoryOutcome,
	)
	inspection.append(repositoryFact, repositoryDiagnostic)
	if repositoryOutcome.State != integrationDoctorVerified {
		for _, workflow := range request.Workflows {
			inspection.append(integrationDoctorRemoteFact(
				workflow.Path, "remote_workflow_identity", integrationDoctorNotAttempted,
				"Remote workflow checks were not attempted because repository access was not established.",
				workflow.Path, "", workflow.Path,
			), nil)
		}
		inspection.finalize()
		return inspection
	}

	remoteContext := &integrationDoctorRemoteContext{
		identity:       request.Identity,
		repository:     repository,
		publicToken:    repositoryToken,
		protected:      tokens,
		variableValues: make(map[string]string),
		releases:       make(map[string]integrationDoctorRemoteReleaseObservation),
		tags:           make(map[string]integrationDoctorRemoteTagObservation),
	}
	inspector.inspectActionsPolicy(ctx, remoteContext, &inspection)
	for _, workflow := range request.Workflows {
		inspector.inspectWorkflow(ctx, remoteContext, workflow, &inspection)
	}
	inspector.inspectVariables(ctx, remoteContext, request.Workflows, &inspection)
	inspector.inspectSecrets(ctx, remoteContext, request.Workflows, &inspection)
	inspector.inspectInstallationArtifacts(ctx, remoteContext, request, &inspection)
	inspector.inspectPublicationTargets(ctx, remoteContext, request, &inspection)
	inspection.finalize()
	return inspection
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectRepository(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	tokens *integrationDoctorRemoteTokenAccess,
) (integrationDoctorGitHubRepository, integrationDoctorGitHubReadOutcome, githubReadToken) {
	repository, outcome := inspector.reader.Repository(ctx, identity, githubReadToken{})
	if outcome.State != integrationDoctorMissing && outcome.State != integrationDoctorUnauthorized {
		return repository, outcome, githubReadToken{}
	}
	token, available := tokens.Resolve(ctx)
	if !available {
		if outcome.State == integrationDoctorMissing {
			outcome.State = integrationDoctorUnauthorized
		}
		return integrationDoctorGitHubRepository{}, outcome, githubReadToken{}
	}
	repository, outcome = inspector.reader.Repository(ctx, identity, token)
	return repository, outcome, token
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectActionsPolicy(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	inspection *integrationDoctorRemoteInspection,
) {
	token, available := remote.protected.Resolve(ctx)
	if !available {
		inspection.append(
			integrationDoctorRemoteFact(
				"actions-policy", "remote_workflow_identity", integrationDoctorUnauthorized,
				"Repository Actions policy requires protected metadata access and no token was available.",
				"", "", ".git/config",
			),
			integrationDoctorRemoteDiagnostic(
				integrationDoctorNotVerifiable, "REMOTE_ACTIONS_POLICY_UNAUTHORIZED", "", "",
				"Repository Actions policy could not be read with the available identity.",
				"Grant read access to Actions repository policy metadata if this optional check is required.",
			),
		)
		return
	}
	policy, outcome := inspector.reader.ActionsPolicy(ctx, remote.identity, token)
	fact := integrationDoctorRemoteFact(
		"actions-policy", "remote_workflow_identity", outcome.State,
		integrationDoctorRemoteOutcomeEvidence("Repository Actions policy", outcome), "", "", ".git/config",
	)
	var diagnostic *integrationDoctorDiagnostic
	if outcome.State == integrationDoctorVerified {
		if !policy.Enabled {
			fact.State = integrationDoctorMismatch
			fact.Evidence = "Repository Actions policy is disabled."
			diagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorError, "REMOTE_ACTIONS_DISABLED", "", "",
				"GitHub Actions is disabled for the repository.",
				"Enable GitHub Actions before relying on Release V2 workflow dispatch.",
			)
		} else if !integrationDoctorActionsPolicySupported(policy.AllowedActions) {
			fact.State = integrationDoctorUnsupported
			fact.Evidence = "Repository Actions policy returned an enabled but unsupported allowed-actions state."
			diagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorNotVerifiable, "REMOTE_ACTIONS_POLICY_UNSUPPORTED", "", "",
				"Repository Actions policy uses an allowed-actions state this Doctor does not claim to understand.",
				"Inspect the exact repository Actions policy without mutating it.",
			)
		} else {
			fact.Evidence = "Repository Actions policy is enabled."
		}
	} else {
		diagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			outcome, "REMOTE_ACTIONS_POLICY", "", "", "repository Actions policy",
		)
	}
	inspection.append(fact, diagnostic)
}

func integrationDoctorActionsPolicySupported(allowedActions string) bool {
	switch allowedActions {
	case "all", "local_only", "selected":
		return true
	default:
		return false
	}
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectWorkflow(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	workflow integrationDoctorRemoteWorkflow,
	inspection *integrationDoctorRemoteInspection,
) {
	content, contentOutcome := inspector.reader.WorkflowContent(
		ctx, remote.identity, workflow.Path, remote.repository.DefaultBranch, remote.publicToken,
	)
	contentFact := integrationDoctorRemoteFact(
		workflow.Path+"#content", "remote_workflow_identity", contentOutcome.State,
		integrationDoctorRemoteOutcomeEvidence("Remote default-branch workflow content", contentOutcome),
		workflow.Path, "", workflow.Path,
	)
	var contentDiagnostic *integrationDoctorDiagnostic
	if contentOutcome.State == integrationDoctorVerified {
		if workflow.Snapshot.Exists && bytes.Equal(content.Content, workflow.Snapshot.Content) {
			contentFact.Evidence = "Remote default-branch workflow bytes exactly match the repository-confined local workflow bytes."
		} else {
			contentFact.State = integrationDoctorMismatch
			contentFact.Evidence = "Remote default-branch workflow bytes differ from the repository-confined local workflow bytes."
			contentDiagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorError, "REMOTE_WORKFLOW_CONTENT_MISMATCH", workflow.Path, "",
				"The remote default-branch workflow does not byte-match the inspected local workflow.",
				"Commit the exact inspected workflow bytes to the remote default branch.",
			)
		}
	} else {
		contentDiagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			contentOutcome, "REMOTE_WORKFLOW_CONTENT", workflow.Path, "", "remote workflow content",
		)
	}
	inspection.append(contentFact, contentDiagnostic)

	metadata, metadataOutcome := inspector.reader.WorkflowMetadata(
		ctx, remote.identity, workflow.Path, remote.publicToken,
	)
	metadataFact := integrationDoctorRemoteFact(
		workflow.Path+"#state", "remote_workflow_identity", metadataOutcome.State,
		integrationDoctorRemoteOutcomeEvidence("Remote workflow enabled state", metadataOutcome),
		workflow.Path, "", workflow.Path,
	)
	var metadataDiagnostic *integrationDoctorDiagnostic
	if metadataOutcome.State == integrationDoctorVerified {
		switch metadata.State {
		case "active":
			metadataFact.Evidence = "The exact configured GitHub Actions workflow is active."
		case "disabled_manually", "disabled_inactivity":
			metadataFact.State = integrationDoctorMismatch
			metadataFact.Evidence = "The exact configured GitHub Actions workflow is disabled with state " + metadata.State + "."
			metadataDiagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorError, "REMOTE_WORKFLOW_DISABLED", workflow.Path, "",
				"The configured GitHub Actions workflow is disabled remotely.",
				"Enable the exact configured workflow before release dispatch.",
			)
		default:
			metadataFact.State = integrationDoctorUnsupported
			metadataFact.Evidence = "GitHub returned an unsupported workflow state without proving the workflow disabled."
			metadataDiagnostic = integrationDoctorRemoteDiagnostic(
				integrationDoctorNotVerifiable, "REMOTE_WORKFLOW_STATE_UNSUPPORTED", workflow.Path, "",
				"The configured workflow returned an unsupported GitHub state.",
				"Inspect the exact workflow state in GitHub without dispatching it.",
			)
		}
	} else {
		metadataDiagnostic = integrationDoctorDiagnosticForRemoteOutcome(
			metadataOutcome, "REMOTE_WORKFLOW_STATE", workflow.Path, "", "remote workflow state",
		)
	}
	inspection.append(metadataFact, metadataDiagnostic)
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectVariables(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	workflows []integrationDoctorRemoteWorkflow,
	inspection *integrationDoctorRemoteInspection,
) {
	references := integrationDoctorRecognizedRemoteVariables(workflows)
	for _, reference := range references {
		token, available := remote.protected.Resolve(ctx)
		if !available {
			inspection.append(
				integrationDoctorRemoteFact(
					reference.Name, "repository_variable_values", integrationDoctorUnauthorized,
					"The recognized repository variable could not be queried without protected metadata access.",
					reference.Workflow, "", reference.Workflow,
				),
				integrationDoctorRemoteDiagnostic(
					integrationDoctorNotVerifiable, "REMOTE_REPOSITORY_VARIABLE_UNAUTHORIZED", reference.Workflow, "",
					fmt.Sprintf("Recognized repository variable %q could not be read with the available identity.", reference.Name),
					"Grant Actions variables read access for the explicitly requested remote verification.",
				),
			)
			continue
		}
		variable, outcome := inspector.reader.RepositoryVariable(ctx, remote.identity, reference.Name, token)
		fact := integrationDoctorRemoteFact(
			reference.Name, "repository_variable_values", outcome.State,
			integrationDoctorRemoteOutcomeEvidence("Recognized repository variable", outcome),
			reference.Workflow, "", reference.Workflow,
		)
		var diagnostic *integrationDoctorDiagnostic
		if outcome.State == integrationDoctorVerified {
			canonical, valid := integrationDoctorCanonicalRemoteVersion(variable.Value)
			if valid {
				fact.Evidence = fmt.Sprintf("Recognized repository variable %s exists with supported canonical version %s.", reference.Name, canonical)
				remote.variableValues[reference.Name] = canonical
			} else {
				fact.State = integrationDoctorMismatch
				fact.Evidence = "The recognized repository variable exists but is not a supported pinned semantic version."
				diagnostic = integrationDoctorRemoteDiagnostic(
					integrationDoctorError, "REMOTE_REPOSITORY_VARIABLE_INVALID", reference.Workflow, "",
					fmt.Sprintf("Recognized repository variable %q does not contain a supported pinned semantic version.", reference.Name),
					"Set the exact recognized variable to a supported semantic version without latest or discovery semantics.",
				)
			}
		} else {
			diagnostic = integrationDoctorDiagnosticForRemoteOutcome(
				outcome, "REMOTE_REPOSITORY_VARIABLE", reference.Workflow, "", "recognized repository variable "+reference.Name,
			)
		}
		inspection.append(fact, diagnostic)
	}
}

func (inspector integrationDoctorGitHubRemoteInspector) inspectSecrets(
	ctx context.Context,
	remote *integrationDoctorRemoteContext,
	workflows []integrationDoctorRemoteWorkflow,
	inspection *integrationDoctorRemoteInspection,
) {
	for _, reference := range integrationDoctorReferencedCustomSecrets(workflows) {
		token, available := remote.protected.Resolve(ctx)
		if !available {
			inspection.append(
				integrationDoctorRemoteFact(
					reference.Name, "credential_wiring", integrationDoctorUnauthorized,
					"Referenced custom secret-name metadata could not be queried without protected metadata access.",
					reference.Workflow, "", reference.Workflow,
				),
				integrationDoctorRemoteDiagnostic(
					integrationDoctorNotVerifiable, "REMOTE_SECRET_METADATA_UNAUTHORIZED", reference.Workflow, "",
					fmt.Sprintf("Referenced custom secret %q metadata could not be read with the available identity.", reference.Name),
					"Grant repository secret metadata read access without exposing the secret value.",
				),
			)
			continue
		}
		metadata, outcome := inspector.reader.RepositorySecret(ctx, remote.identity, reference.Name, token)
		fact := integrationDoctorRemoteFact(
			reference.Name, "credential_wiring", outcome.State,
			integrationDoctorRemoteOutcomeEvidence("Referenced custom secret-name metadata", outcome),
			reference.Workflow, "", reference.Workflow,
		)
		var diagnostic *integrationDoctorDiagnostic
		if outcome.State == integrationDoctorVerified {
			fact.Evidence = fmt.Sprintf("Referenced custom secret name %s exists; no value was requested or returned.", metadata.Name)
		} else {
			diagnostic = integrationDoctorDiagnosticForRemoteOutcome(
				outcome, "REMOTE_SECRET_METADATA", reference.Workflow, "", "referenced custom secret metadata "+reference.Name,
			)
		}
		inspection.append(fact, diagnostic)
	}
}

type integrationDoctorNamedRemoteReference struct {
	Name     string
	Workflow string
}

func integrationDoctorRecognizedRemoteVariables(
	workflows []integrationDoctorRemoteWorkflow,
) []integrationDoctorNamedRemoteReference {
	seen := make(map[string]integrationDoctorNamedRemoteReference)
	for _, workflow := range workflows {
		root := workflowDocumentRoot(workflow.Snapshot.Document)
		jobs := integrationDoctorWorkflowJobs(root)
		//nolint:govet // Testable recognition policy reads name before its predicate.
		for _, requirement := range []struct {
			name    string
			matches func(integrationDoctorWorkflowStep) bool
		}{
			{name: "NEKO_VERSION", matches: func(step integrationDoctorWorkflowStep) bool { return strings.Contains(step.run, "install.sh") }},
			{name: "NEKO_RELEASE_PLUGIN_VERSION", matches: func(step integrationDoctorWorkflowStep) bool {
				return strings.Contains(step.run, "neko plugin install release")
			}},
		} {
			step, present := integrationDoctorInstallationStep(jobs, requirement.matches)
			if present && integrationDoctorPinnedRepositoryVariable(step, requirement.name) {
				existing, found := seen[requirement.name]
				if !found || workflow.Path < existing.Workflow {
					seen[requirement.name] = integrationDoctorNamedRemoteReference{Name: requirement.name, Workflow: workflow.Path}
				}
			}
		}
	}
	return integrationDoctorSortedRemoteReferences(seen)
}

func integrationDoctorReferencedCustomSecrets(
	workflows []integrationDoctorRemoteWorkflow,
) []integrationDoctorNamedRemoteReference {
	seen := make(map[string]integrationDoctorNamedRemoteReference)
	for _, workflow := range workflows {
		root := workflowDocumentRoot(workflow.Snapshot.Document)
		for _, reference := range integrationDoctorCredentialReferences(root, integrationDoctorWorkflowJobs(root)) {
			if reference.Kind != integrationDoctorCustomCredential || reference.Name == "GITHUB_TOKEN" {
				continue
			}
			existing, found := seen[reference.Name]
			if !found || workflow.Path < existing.Workflow {
				seen[reference.Name] = integrationDoctorNamedRemoteReference{Name: reference.Name, Workflow: workflow.Path}
			}
		}
	}
	return integrationDoctorSortedRemoteReferences(seen)
}

func integrationDoctorSortedRemoteReferences(
	values map[string]integrationDoctorNamedRemoteReference,
) []integrationDoctorNamedRemoteReference {
	references := make([]integrationDoctorNamedRemoteReference, 0, len(values))
	for _, reference := range values {
		references = append(references, reference)
	}
	sort.Slice(references, func(left, right int) bool {
		return references[left].Name+"\x00"+references[left].Workflow < references[right].Name+"\x00"+references[right].Workflow
	})
	return references
}

func integrationDoctorCanonicalRemoteVersion(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "latest") || strings.Contains(value, "/") {
		return "", false
	}
	value = strings.TrimPrefix(value, "v")
	canonical, err := releaseconfig.CanonicalReleaseVersion(value)
	return canonical, err == nil
}

func (inspection *integrationDoctorRemoteInspection) append(
	fact integrationDoctorVerification,
	diagnostic *integrationDoctorDiagnostic,
) {
	inspection.Verifications = append(inspection.Verifications, fact)
	if diagnostic != nil {
		inspection.Diagnostics = append(inspection.Diagnostics, *diagnostic)
	}
}

func (inspection *integrationDoctorRemoteInspection) finalize() {
	for _, fact := range inspection.Verifications {
		switch fact.State {
		case integrationDoctorVerified:
			inspection.Summary.Verified++
		case integrationDoctorMissing, integrationDoctorMismatch:
			inspection.Summary.Failed++
		default:
			inspection.Summary.Unresolved++
		}
	}
	switch {
	case inspection.Summary.Unresolved == 0:
		inspection.Summary.Status = integrationDoctorRemoteComplete
	case inspection.Summary.Verified == 0 && inspection.Summary.Failed == 0:
		inspection.Summary.Status = integrationDoctorRemoteUnavailable
	default:
		inspection.Summary.Status = integrationDoctorRemotePartial
	}
}

var _ integrationDoctorRemoteInspector = integrationDoctorGitHubRemoteInspector{}
