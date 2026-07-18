package release

import (
	"bytes"
	"fmt"
	"text/template"
)

// GitHubActionsReleaseWorkflowContractVersion identifies the generated
// workflow shape. It is independent from Release V2 config and plugin response
// schema versions.
const GitHubActionsReleaseWorkflowContractVersion = 1

type githubActionsReleaseWorkflowInput struct {
	Name        string
	Description string
}

type githubActionsReleaseWorkflowSpec struct {
	Name                  string
	CheckoutAction        string
	ValidationStepID      string
	ConsumerCommand       string
	Inputs                []githubActionsReleaseWorkflowInput
	ContractVersion       int
	CancelReleaseInFlight bool
}

func canonicalGitHubActionsReleaseWorkflowSpec() githubActionsReleaseWorkflowSpec {
	return githubActionsReleaseWorkflowSpec{
		Name:             "Release selected unit",
		CheckoutAction:   "actions/checkout@v4",
		ValidationStepID: "release-context",
		ConsumerCommand:  "./tooling/publish-release",
		Inputs: []githubActionsReleaseWorkflowInput{
			{Name: "unit", Description: "Neko Release V2 unit id"},
			{Name: "version", Description: "Neko-authoritative release version"},
			{Name: "tag", Description: "Neko-created unit tag"},
			{Name: "release_sha", Description: "Neko-created release commit SHA"},
		},
		ContractVersion:       GitHubActionsReleaseWorkflowContractVersion,
		CancelReleaseInFlight: false,
	}
}

// RenderCanonicalGitHubActionsReleaseWorkflow renders the deterministic
// build-system-neutral workflow shared by documentation and scaffolding.
func RenderCanonicalGitHubActionsReleaseWorkflow() ([]byte, error) {
	spec := canonicalGitHubActionsReleaseWorkflowSpec()
	if err := validateGitHubActionsReleaseWorkflowSpec(spec); err != nil {
		return nil, err
	}

	workflowTemplate, err := template.New("github-actions-release-workflow").
		Delims("[[", "]]").
		Parse(canonicalGitHubActionsReleaseWorkflowTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse canonical GitHub Actions release workflow template: %w", err)
	}
	var output bytes.Buffer
	if err := workflowTemplate.Execute(&output, spec); err != nil {
		return nil, fmt.Errorf("render canonical GitHub Actions release workflow: %w", err)
	}
	return output.Bytes(), nil
}

func validateGitHubActionsReleaseWorkflowSpec(spec githubActionsReleaseWorkflowSpec) error {
	if spec.ContractVersion != GitHubActionsReleaseWorkflowContractVersion {
		return fmt.Errorf("unsupported GitHub Actions release workflow contract version %d", spec.ContractVersion)
	}
	if spec.Name == "" || spec.CheckoutAction == "" || spec.ValidationStepID == "" || spec.ConsumerCommand == "" {
		return fmt.Errorf("canonical GitHub Actions release workflow specification is incomplete")
	}
	expectedInputs := []string{"unit", "version", "tag", "release_sha"}
	if len(spec.Inputs) != len(expectedInputs) {
		return fmt.Errorf("canonical GitHub Actions release workflow requires exactly four inputs")
	}
	for index, expected := range expectedInputs {
		if spec.Inputs[index].Name != expected || spec.Inputs[index].Description == "" {
			return fmt.Errorf("canonical GitHub Actions release workflow input %d must be %q with a description", index, expected)
		}
	}
	return nil
}

const canonicalGitHubActionsReleaseWorkflowTemplate = `name: [[ .Name ]]

on:
  workflow_dispatch:
    inputs:
[[ range .Inputs ]]      [[ .Name ]]:
        description: [[ .Description ]]
        required: true
        type: string
[[ end ]]
permissions:
  contents: read

concurrency:
  group: release-${{ inputs.unit }}-${{ inputs.tag }}
  cancel-in-progress: [[ .CancelReleaseInFlight ]]

jobs:
  release:
    runs-on: ubuntu-latest
    env:
      RELEASE_UNIT: ${{ inputs.unit }}
      RELEASE_VERSION: ${{ inputs.version }}
      RELEASE_TAG: ${{ inputs.tag }}
      RELEASE_SHA: ${{ inputs.release_sha }}
    steps:
      - name: Checkout the exact release commit with tags
        uses: [[ .CheckoutAction ]]
        with:
          ref: ${{ inputs.release_sha }}
          fetch-depth: 0
          fetch-tags: true
          persist-credentials: false

      - name: Validate Neko release context
        id: [[ .ValidationStepID ]]
        shell: bash
        env:
          RELEASE_UNIT: ${{ inputs.unit }}
          RELEASE_VERSION: ${{ inputs.version }}
          RELEASE_TAG: ${{ inputs.tag }}
          RELEASE_SHA: ${{ inputs.release_sha }}
        run: |
          set -euo pipefail
          neko release ci-validate-context \
            --unit "$RELEASE_UNIT" \
            --version "$RELEASE_VERSION" \
            --tag "$RELEASE_TAG" \
            --release-sha "$RELEASE_SHA" \
            --output github \
            --github-output-file "$GITHUB_OUTPUT"

      # Consumer-owned extension point. Replace this command with a script that
      # builds and publishes only RELEASE_UNIT at RELEASE_VERSION. Pass secrets
      # in this step's env and add only its required job permissions.
      - name: Build and publish selected unit
        shell: bash
        env:
          RELEASE_UNIT: ${{ steps.[[ .ValidationStepID ]].outputs.unit }}
          RELEASE_VERSION: ${{ steps.[[ .ValidationStepID ]].outputs.version }}
          RELEASE_TAG: ${{ steps.[[ .ValidationStepID ]].outputs.tag }}
          RELEASE_SHA: ${{ steps.[[ .ValidationStepID ]].outputs.release_sha }}
        run: |
          set -euo pipefail
          [[ .ConsumerCommand ]] \
            --unit "$RELEASE_UNIT" \
            --version "$RELEASE_VERSION" \
            --tag "$RELEASE_TAG" \
            --release-sha "$RELEASE_SHA"
`
