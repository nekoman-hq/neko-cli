package release

import "fmt"

type DispatchJournalState string

const (
	DispatchJournalPrepared       DispatchJournalState = "prepared"
	DispatchJournalRequestStarted DispatchJournalState = "request-started"
	DispatchJournalAccepted       DispatchJournalState = "accepted"
	DispatchJournalRejected       DispatchJournalState = "rejected"
	DispatchJournalUnknown        DispatchJournalState = "unknown"
)

func (state DispatchJournalState) Valid() bool {
	switch state {
	case DispatchJournalPrepared, DispatchJournalRequestStarted, DispatchJournalAccepted, DispatchJournalRejected, DispatchJournalUnknown:
		return true
	default:
		return false
	}
}

func (state DispatchJournalState) CanTransitionTo(next DispatchJournalState) bool {
	if !state.Valid() || !next.Valid() {
		return false
	}
	switch state {
	case DispatchJournalPrepared:
		return next == DispatchJournalRequestStarted
	case DispatchJournalRequestStarted:
		return next == DispatchJournalAccepted || next == DispatchJournalRejected || next == DispatchJournalUnknown
	case DispatchJournalAccepted, DispatchJournalRejected, DispatchJournalUnknown:
		return false
	default:
		return false
	}
}

func validateDispatchJournalTransition(current, next DispatchJournalState) error {
	if !current.Valid() {
		return fmt.Errorf("dispatch journal state %q is invalid", current)
	}
	if !next.Valid() {
		return fmt.Errorf("dispatch journal target state %q is invalid", next)
	}
	if !current.CanTransitionTo(next) {
		return fmt.Errorf("dispatch journal transition %s -> %s is invalid", current, next)
	}
	return nil
}

func dispatchJournalRecoveryGuidance(state DispatchJournalState) string {
	switch state {
	case DispatchJournalPrepared:
		return "Dispatch request is prepared locally. No HTTP request has been attempted."
	case DispatchJournalRequestStarted:
		return "Dispatch request may be in flight or may have reached GitHub. Do not automatically retry; inspect GitHub Actions before any future resume."
	case DispatchJournalAccepted:
		return "Dispatch was accepted by GitHub in a future-capable run. Do not redispatch automatically."
	case DispatchJournalRejected:
		return "Dispatch was rejected by GitHub in a future-capable run. Preserve the journal for diagnosis."
	case DispatchJournalUnknown:
		return "The dispatch outcome is uncertain. Do not blindly dispatch again. Inspect the configured GitHub Actions workflow and this journal before any future explicit resume action."
	default:
		return "Dispatch journal state is invalid. Manual inspection is required."
	}
}
