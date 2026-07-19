package release

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type integrationDoctorPermissions struct {
	scopes     map[string]string
	explicit   bool
	understood bool
	writeAll   bool
}

func inspectIntegrationDoctorPermissions(
	root *yaml.Node,
	add func(integrationDoctorSeverity, string, string, string),
) {
	workflowNode := workflowMappingValue(root, "permissions")
	jobs := integrationDoctorPermissionJobs(root)
	if integrationDoctorPermissionsAreImplicit(workflowNode, jobs) {
		add(integrationDoctorWarning, "PERMISSIONS_IMPLICIT", "Workflow and jobs rely on implicit GitHub token permissions.", "Declare least-privilege permissions explicitly at workflow or job scope.")
	}

	broad := integrationDoctorWorkflowPermissionsAreUnsafe(parseIntegrationDoctorPermissions(workflowNode))
	for _, job := range jobs {
		if job.permissions != nil && integrationDoctorJobPermissionsAreUnsafe(job, parseIntegrationDoctorPermissions(job.permissions)) {
			broad = true
		}
	}
	if broad {
		add(integrationDoctorWarning, "PERMISSIONS_BROAD", "The workflow grants broad, unsupported, or unjustified permissions.", "Keep a read-only workflow default and grant supported write scopes only in jobs with matching publication operations.")
	}
}

func integrationDoctorPermissionJobs(root *yaml.Node) []integrationDoctorWorkflowJob {
	jobsWithSteps := integrationDoctorWorkflowJobs(root)
	jobsByID := make(map[string]integrationDoctorWorkflowJob, len(jobsWithSteps))
	for _, job := range jobsWithSteps {
		jobsByID[job.id] = job
	}

	jobsNode := workflowMappingValue(root, "jobs")
	jobs := make([]integrationDoctorWorkflowJob, 0, len(workflowMappingKeys(jobsNode)))
	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return jobs
	}
	for index := 0; index+1 < len(jobsNode.Content); index += 2 {
		id := jobsNode.Content[index].Value
		if job, ok := jobsByID[id]; ok {
			jobs = append(jobs, job)
			continue
		}
		jobNode := jobsNode.Content[index+1]
		jobs = append(jobs, integrationDoctorWorkflowJob{
			id:          id,
			node:        jobNode,
			permissions: workflowMappingValue(jobNode, "permissions"),
			env:         workflowMappingValue(jobNode, "env"),
		})
	}
	return jobs
}

func integrationDoctorPermissionsAreImplicit(
	workflowNode *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
) bool {
	if workflowNode != nil {
		return false
	}
	if len(jobs) == 0 {
		return true
	}
	for _, job := range jobs {
		if job.permissions == nil {
			return true
		}
	}
	return false
}

func parseIntegrationDoctorPermissions(node *yaml.Node) integrationDoctorPermissions {
	permissions := integrationDoctorPermissions{
		explicit:   node != nil,
		understood: true,
	}
	if node == nil {
		return permissions
	}
	if node.Kind == yaml.ScalarNode {
		switch node.Value {
		case "read-all":
			return permissions
		case "write-all":
			permissions.writeAll = true
			return permissions
		default:
			permissions.understood = false
			return permissions
		}
	}
	if node.Kind != yaml.MappingNode || len(node.Content)%2 != 0 {
		permissions.understood = false
		return permissions
	}

	permissions.scopes = make(map[string]string, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		scopeNode := node.Content[index]
		accessNode := node.Content[index+1]
		scope := scopeNode.Value
		access := accessNode.Value
		_, duplicate := permissions.scopes[scope]
		if scopeNode.Kind != yaml.ScalarNode || accessNode.Kind != yaml.ScalarNode ||
			!integrationDoctorPermissionScopeSupported(scope) ||
			!integrationDoctorPermissionAccessSupported(scope, access) || duplicate {
			permissions.understood = false
			continue
		}
		permissions.scopes[scope] = access
	}
	return permissions
}

func integrationDoctorPermissionScopeSupported(scope string) bool {
	switch scope {
	case "actions", "artifact-metadata", "attestations", "checks", "code-quality", "contents",
		"deployments", "discussions", "id-token", "issues", "models", "packages", "pages",
		"pull-requests", "security-events", "statuses", "vulnerability-alerts":
		return true
	default:
		return false
	}
}

func integrationDoctorPermissionAccessSupported(scope, access string) bool {
	if access == "none" {
		return true
	}
	if scope == "id-token" {
		return access == "write"
	}
	if scope == "models" || scope == "vulnerability-alerts" {
		return access == "read"
	}
	return access == "read" || access == "write"
}

func integrationDoctorWorkflowPermissionsAreUnsafe(permissions integrationDoctorPermissions) bool {
	if !permissions.explicit {
		return false
	}
	return !permissions.understood || permissions.writeAll || integrationDoctorPermissionsContainWrite(permissions)
}

func integrationDoctorJobPermissionsAreUnsafe(
	job integrationDoctorWorkflowJob,
	permissions integrationDoctorPermissions,
) bool {
	if !permissions.understood || permissions.writeAll {
		return true
	}
	for scope, access := range permissions.scopes {
		if access == "write" && !integrationDoctorJobSupportsPermissionWrite(job, scope) {
			return true
		}
	}
	return false
}

func integrationDoctorPermissionsContainWrite(permissions integrationDoctorPermissions) bool {
	for _, access := range permissions.scopes {
		if access == "write" {
			return true
		}
	}
	return false
}

func integrationDoctorJobSupportsPermissionWrite(job integrationDoctorWorkflowJob, scope string) bool {
	switch scope {
	case "contents":
		return integrationDoctorJobPublishesRepositoryContents(job)
	case "packages":
		return integrationDoctorJobPublishesGitHubPackage(job)
	default:
		return false
	}
}

func integrationDoctorJobPublishesRepositoryContents(job integrationDoctorWorkflowJob) bool {
	for _, step := range job.steps {
		if integrationDoctorGoReleaserStepPublishes(step) || integrationDoctorRunPublishesGitHubRelease(step.run) {
			return true
		}
	}
	return false
}

func integrationDoctorGoReleaserStepPublishes(step integrationDoctorWorkflowStep) bool {
	if !strings.HasPrefix(strings.ToLower(step.uses), "goreleaser/goreleaser-action@") {
		return false
	}
	args := workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "with"), "args"))
	fields := strings.Fields(args)
	if len(fields) == 0 || fields[0] != "release" {
		return false
	}
	for index, field := range fields {
		field = strings.Trim(field, "'\"")
		if field == "--snapshot" || field == "--snapshot=true" {
			return false
		}
		if strings.HasPrefix(field, "--skip=") && integrationDoctorCommaListContains(strings.TrimPrefix(field, "--skip="), "publish") {
			return false
		}
		if field == "--skip" && index+1 < len(fields) && integrationDoctorCommaListContains(fields[index+1], "publish") {
			return false
		}
	}
	return true
}

func integrationDoctorRunPublishesGitHubRelease(command string) bool {
	for _, line := range strings.Split(command, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "gh" && fields[1] == "release" &&
			(fields[2] == "create" || fields[2] == "upload") {
			return true
		}
	}
	return false
}

func integrationDoctorJobPublishesGitHubPackage(job integrationDoctorWorkflowJob) bool {
	for _, step := range job.steps {
		if integrationDoctorRunPushesGitHubContainer(step.run) || integrationDoctorBuildPushStepPublishesGitHubContainer(step) {
			return true
		}
	}
	return false
}

func integrationDoctorRunPushesGitHubContainer(command string) bool {
	for _, line := range strings.Split(command, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 || fields[0] != "docker" || fields[1] != "push" {
			continue
		}
		image := strings.Trim(fields[2], "'\"")
		if strings.HasPrefix(strings.ToLower(image), "ghcr.io/") {
			return true
		}
	}
	return false
}

func integrationDoctorBuildPushStepPublishesGitHubContainer(step integrationDoctorWorkflowStep) bool {
	if !strings.HasPrefix(strings.ToLower(step.uses), "docker/build-push-action@") {
		return false
	}
	with := workflowMappingValue(step.node, "with")
	push, pushOK := workflowBool(workflowMappingValue(with, "push"))
	return pushOK && push && integrationDoctorListContainsGitHubContainer(workflowScalar(workflowMappingValue(with, "tags")))
}

func integrationDoctorListContainsGitHubContainer(value string) bool {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	for _, field := range fields {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(field)), "ghcr.io/") {
			return true
		}
	}
	return false
}

func integrationDoctorCommaListContains(value, want string) bool {
	for _, item := range strings.Split(strings.Trim(value, "'\""), ",") {
		if item == want {
			return true
		}
	}
	return false
}
