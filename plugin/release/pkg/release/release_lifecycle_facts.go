package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"

// configuredReleaseLifecycleStages returns a fresh immutable description of
// the direct-call root lifecycle. It is descriptive only and owns no handlers.
func configuredReleaseLifecycleStages() []pipelineinspection.LifecycleStage {
	return []pipelineinspection.LifecycleStage{
		{
			ID: "source-unit-resolution", Label: "Resolve release source and unit",
			Owner: "Neko CLI", Location: "local process", Mutation: "none",
			ConfigurationStatus: "configured", Source: "pkg/release/release_start_v2.go",
		},
		{
			ID: "release-context-planning", Label: "Plan release identity during execution",
			Owner: "Neko CLI", Location: "local process", Mutation: "none",
			ConfigurationStatus: "configured", Source: "pkg/release/release_start_v2.go",
		},
		{
			ID: "dispatch-token-resolution", Label: "Resolve workflow dispatch token",
			Owner: "Neko CLI", Location: "local process", Mutation: "none",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "release-file-planning", Label: "Plan materialized and known release files",
			Owner: "Neko CLI", Location: "local repository", Mutation: "none",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "release-preflight", Label: "Validate local Git release preflight",
			Owner: "Neko CLI", Location: "local Git", Mutation: "none",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "execution-journal-preparation", Label: "Prepare execution journal intent",
			Owner: "Neko CLI", Location: "local repository", Mutation: "release state",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "release-file-materialization", Label: "Materialize configured release files",
			Owner: "Neko CLI", Location: "local repository", Mutation: "filesystem",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "selected-unit-state-write", Label: "Write selected unit release state",
			Owner: "Neko CLI", Location: "local repository", Mutation: "release state",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "known-release-file-staging", Label: "Stage known release files",
			Owner: "local Git", Location: "local Git", Mutation: "Git index",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "release-commit-creation", Label: "Create release commit",
			Owner: "local Git", Location: "local Git", Mutation: "Git object",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "unit-tag-creation", Label: "Create unit tag",
			Owner: "local Git", Location: "local Git", Mutation: "Git ref",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "workflow-request-preparation", Label: "Prepare workflow request and dispatch journal",
			Owner: "Neko CLI", Location: "local repository", Mutation: "release state",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "release-commit-push", Label: "Push release commit",
			Owner: "remote Git", Location: "remote Git", Mutation: "remote Git",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "unit-tag-push", Label: "Push unit tag",
			Owner: "remote Git", Location: "remote Git", Mutation: "remote Git",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "workflow-request-submission", Label: "Submit workflow dispatch request",
			Owner: "GitHub API", Location: "GitHub API", Mutation: "remote API",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
		{
			ID: "handoff-confirmation", Label: "Confirm workflow handoff",
			Owner: "Neko CLI", Location: "local repository", Mutation: "release state",
			ConfigurationStatus: "configured", Source: "pkg/release/github_actions_release_use_case.go",
		},
	}
}
