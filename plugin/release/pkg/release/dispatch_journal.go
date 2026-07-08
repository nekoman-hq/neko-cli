package release

import "time"

const dispatchJournalSchemaVersion = 1

// DispatchJournal is the durable record for a future GitHub Actions dispatch.
// It stores release facts and state only; secrets, tokens and authorization
// values must never be written to this model.
//
//nolint:govet // Journal fields follow persisted JSON order.
type DispatchJournal struct {
	SchemaVersion        int                     `json:"schemaVersion"`
	Identity             ReleaseDispatchIdentity `json:"identity"`
	RepositoryRemoteName string                  `json:"repositoryRemoteName"`
	RepositoryRemote     string                  `json:"repositoryRemote"`
	UnitID               string                  `json:"unit"`
	Version              string                  `json:"version"`
	Tag                  string                  `json:"tag"`
	ReleaseCommitSHA     string                  `json:"releaseCommitSHA"`
	WorkflowPath         string                  `json:"workflowPath"`
	WorkflowFileName     string                  `json:"workflowFileName"`
	Executor             string                  `json:"executor"`
	Delivery             string                  `json:"delivery"`
	Inputs               map[string]string       `json:"inputs"`
	State                DispatchJournalState    `json:"state"`
	CreatedAt            time.Time               `json:"createdAt"`
	UpdatedAt            time.Time               `json:"updatedAt"`
	LastError            string                  `json:"lastError,omitempty"`
	DispatchMetadata     DispatchJournalMetadata `json:"dispatchMetadata"`
	RecoveryGuidance     string                  `json:"recoveryGuidance"`
}

// DispatchJournalMetadata is intentionally empty-friendly for the future
// GitHub response fields such as workflow run ID, URL, status and request time.
//
//nolint:govet // Metadata field order follows future GitHub response lifecycle.
type DispatchJournalMetadata struct {
	RunID             string    `json:"runId,omitempty"`
	RunURL            string    `json:"runUrl,omitempty"`
	HTMLURL           string    `json:"htmlUrl,omitempty"`
	ResponseStatus    string    `json:"responseStatus,omitempty"`
	ResponseTimestamp string    `json:"responseTimestamp,omitempty"`
	RequestStartedAt  time.Time `json:"requestStartedAt,omitempty"`
	RequestFinishedAt time.Time `json:"requestFinishedAt,omitempty"`
}

func NewPreparedDispatchJournal(request *ReleaseDispatchRequest, now time.Time) (*DispatchJournal, error) {
	if request == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	journal := &DispatchJournal{
		SchemaVersion:        dispatchJournalSchemaVersion,
		Identity:             request.Identity,
		RepositoryRemoteName: request.RepositoryRemoteName,
		RepositoryRemote:     request.Identity.RepositoryRemote,
		UnitID:               request.UnitID,
		Version:              request.Version,
		Tag:                  request.Tag,
		ReleaseCommitSHA:     request.ReleaseCommitSHA,
		WorkflowPath:         request.WorkflowPath,
		WorkflowFileName:     request.WorkflowFileName,
		Executor:             request.Executor,
		Delivery:             request.Delivery,
		Inputs:               cloneDispatchInputs(request.Inputs),
		State:                DispatchJournalPrepared,
		CreatedAt:            now.UTC(),
		UpdatedAt:            now.UTC(),
		DispatchMetadata:     DispatchJournalMetadata{},
		RecoveryGuidance:     dispatchJournalRecoveryGuidance(DispatchJournalPrepared),
	}
	if err := journal.ValidateForRequest(request); err != nil {
		return nil, err
	}
	return journal, nil
}

func (journal *DispatchJournal) ValidateForRequest(request *ReleaseDispatchRequest) error {
	if journal == nil {
		return errDispatchJournalMissing()
	}
	if request == nil {
		return errDispatchRequestMissing()
	}
	if journal.SchemaVersion != dispatchJournalSchemaVersion {
		return errDispatchJournal("schemaVersion mismatch")
	}
	if !journal.State.Valid() {
		return errDispatchJournal("invalid state")
	}
	if journal.Identity.SHA256 != request.Identity.SHA256 {
		return errDispatchJournal("identity mismatch")
	}
	if journal.RepositoryRemoteName != request.RepositoryRemoteName ||
		journal.RepositoryRemote != request.Identity.RepositoryRemote ||
		journal.UnitID != request.UnitID ||
		journal.Version != request.Version ||
		journal.Tag != request.Tag ||
		journal.ReleaseCommitSHA != request.ReleaseCommitSHA ||
		journal.WorkflowPath != request.WorkflowPath ||
		journal.WorkflowFileName != request.WorkflowFileName ||
		journal.Executor != request.Executor ||
		journal.Delivery != request.Delivery {
		return errDispatchJournal("request fields do not match journal identity")
	}
	if !sameStringMap(journal.Inputs, request.Inputs) {
		return errDispatchJournal("inputs differ from request")
	}
	return nil
}

func (journal *DispatchJournal) Transition(next DispatchJournalState, now time.Time, lastError string) error {
	if err := validateDispatchJournalTransition(journal.State, next); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	journal.State = next
	journal.UpdatedAt = now.UTC()
	journal.LastError = lastError
	journal.RecoveryGuidance = dispatchJournalRecoveryGuidance(next)
	return nil
}

func cloneDispatchInputs(inputs map[string]string) map[string]string {
	clone := make(map[string]string, len(inputs))
	for key, value := range inputs {
		clone[key] = value
	}
	return clone
}

func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

type dispatchJournalError string

func (err dispatchJournalError) Error() string {
	return string(err)
}

func errDispatchJournalMissing() error {
	return dispatchJournalError("dispatch journal is missing")
}

func errDispatchRequestMissing() error {
	return dispatchJournalError("dispatch request is missing")
}

func errDispatchJournal(message string) error {
	return dispatchJournalError("dispatch journal " + message)
}
