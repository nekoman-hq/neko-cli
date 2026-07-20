package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// MapResumeCommandOutcome renders one typed resume application outcome.
func MapResumeCommandOutcome(outcome ResumeCommandOutcome, timestamp time.Time) (*plugin.Response, error) {
	switch result := outcome.(type) {
	case *ResumeAssessment:
		return successTableResponse("resume", timestamp, []map[string]any{
			{"property": "Unit", "value": result.UnitID},
			{"property": "Version", "value": result.NextVersion},
			{"property": "Tag", "value": result.Tag},
			{"property": "Execution Journal", "value": result.ExecutionJournalPath},
			{"property": "State", "value": string(result.State)},
			{"property": "Pending Action", "value": string(result.PendingAction)},
			{"property": "Recovery Status", "value": string(result.RecoveryStatus)},
			{"property": "Safe To Continue", "value": fmt.Sprintf("%t", result.SafeToContinue)},
			{"property": "Known Files", "value": strings.Join(result.KnownFilePaths, ", ")},
			{"property": "Next Step", "value": result.Guidance},
		}), nil
	case *GitHubActionsReleaseResult:
		return mapGitHubActionsReleaseResult("resume", result, timestamp), nil
	default:
		return nil, fmt.Errorf("unsupported resume command outcome %T", outcome)
	}
}
