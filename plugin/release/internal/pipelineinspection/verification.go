package pipelineinspection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// VerificationStatus is the Pipeline vocabulary for one neutral verification
// fact. It is intentionally independent from the release lifecycle status.
type VerificationStatus string

const (
	VerificationVerified     VerificationStatus = "verified"
	VerificationFailed       VerificationStatus = "failed"
	VerificationUnavailable  VerificationStatus = "unavailable"
	VerificationUnauthorized VerificationStatus = "unauthorized"
	VerificationRateLimited  VerificationStatus = "rate_limited"
	VerificationNotChecked   VerificationStatus = "not_checked"
	VerificationUnresolved   VerificationStatus = "unresolved"
)

// VerificationClass identifies the boundary needed to establish a fact.
type VerificationClass string

const (
	VerificationLocal            VerificationClass = "local"
	VerificationRemote           VerificationClass = "remote"
	VerificationRuntimeRequired  VerificationClass = "runtime_required"
	VerificationMutationRequired VerificationClass = "mutation_required"
)

// VerificationFact is an immutable neutral input supplied by pkg/release.
// ID is derived by pipelineinspection and must be left empty by callers.
//
//nolint:govet // Field order follows the stable machine contract.
type VerificationFact struct {
	ID         string             `json:"id"`
	Category   string             `json:"category"`
	Class      VerificationClass  `json:"class"`
	Status     VerificationStatus `json:"status"`
	Subject    string             `json:"subject"`
	Evidence   string             `json:"evidence"`
	Source     string             `json:"source"`
	Scope      string             `json:"scope"`
	References []string           `json:"references"`
	Unit       string             `json:"unit,omitempty"`
	Workflow   string             `json:"workflow,omitempty"`
}

// VerificationSnapshot is the read-only composition input. Remote fields
// describe verification only and never alter lifecycle state.
type VerificationSnapshot struct {
	Facts           []VerificationFact
	RemoteStatus    string
	RemoteRequested bool
	RemoteAttempted bool
}

type verificationSummaryStatus string

const (
	verificationSummaryVerified   verificationSummaryStatus = "verified"
	verificationSummaryPartial    verificationSummaryStatus = "partial"
	verificationSummaryFailed     verificationSummaryStatus = "failed"
	verificationSummaryUnresolved verificationSummaryStatus = "unresolved"
	verificationSummaryNotChecked verificationSummaryStatus = "not_checked"
)

//nolint:govet // Field order follows the append-only schema-version-one contract.
type pipelineVerificationSummary struct {
	Status          verificationSummaryStatus `json:"status"`
	LocalStatus     verificationSummaryStatus `json:"local_status"`
	RemoteStatus    string                    `json:"remote_status"`
	RemoteRequested bool                      `json:"remote_requested"`
	RemoteAttempted bool                      `json:"remote_attempted"`
	Partial         bool                      `json:"partial"`
	Verified        int                       `json:"verified"`
	Unresolved      int                       `json:"unresolved"`
	Failed          int                       `json:"failed"`
	NotChecked      int                       `json:"not_checked"`
}

type pipelineVerification struct {
	Summary pipelineVerificationSummary `json:"summary"`
	Facts   []VerificationFact          `json:"facts"`
}

func projectPipelineVerification(snapshot VerificationSnapshot) pipelineVerification {
	facts := cloneVerificationFacts(snapshot.Facts)
	for index := range facts {
		facts[index].ID = pipelineVerificationFactID(facts[index])
	}
	sort.SliceStable(facts, func(left, right int) bool {
		a := facts[left]
		b := facts[right]
		return strings.Join([]string{a.Category, string(a.Class), a.Subject, a.Unit, a.Workflow, string(a.Status), a.ID}, "\x00") <
			strings.Join([]string{b.Category, string(b.Class), b.Subject, b.Unit, b.Workflow, string(b.Status), b.ID}, "\x00")
	})
	summary := summarizePipelineVerification(facts, snapshot)
	return pipelineVerification{Summary: summary, Facts: facts}
}

func cloneVerificationSnapshot(snapshot VerificationSnapshot) VerificationSnapshot {
	clone := snapshot
	clone.Facts = cloneVerificationFacts(snapshot.Facts)
	return clone
}

func cloneVerificationFacts(facts []VerificationFact) []VerificationFact {
	cloned := make([]VerificationFact, 0, len(facts))
	for _, fact := range facts {
		fact.ID = ""
		fact.References = normalizedVerificationReferences(fact.References)
		cloned = append(cloned, fact)
	}
	return cloned
}

func normalizedVerificationReferences(references []string) []string {
	unique := make(map[string]struct{}, len(references))
	normalized := make([]string, 0, len(references))
	for _, reference := range references {
		if reference == "" {
			continue
		}
		if _, exists := unique[reference]; exists {
			continue
		}
		unique[reference] = struct{}{}
		normalized = append(normalized, reference)
	}
	sort.Strings(normalized)
	return normalized
}

func pipelineVerificationFactID(fact VerificationFact) string {
	identity := strings.Join([]string{
		fact.Source, fact.Scope, fact.Category, fact.Subject, fact.Unit, fact.Workflow,
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "verification-" + hex.EncodeToString(digest[:])
}

func summarizePipelineVerification(
	facts []VerificationFact,
	snapshot VerificationSnapshot,
) pipelineVerificationSummary {
	summary := pipelineVerificationSummary{
		RemoteStatus: snapshot.RemoteStatus, RemoteRequested: snapshot.RemoteRequested,
		RemoteAttempted: snapshot.RemoteAttempted,
	}
	if summary.RemoteStatus == "" {
		summary.RemoteStatus = "not_requested"
	}
	localCounts := pipelineVerificationCounts{}
	for _, fact := range facts {
		summary.add(fact.Status)
		if fact.Class != VerificationRemote {
			localCounts.add(fact.Status)
		}
	}
	summary.Status = pipelineVerificationSummaryStatus(
		summary.Verified, summary.Unresolved, summary.Failed, summary.NotChecked,
	)
	summary.LocalStatus = pipelineVerificationSummaryStatus(
		localCounts.verified, localCounts.unresolved, localCounts.failed, localCounts.notChecked,
	)
	summary.Partial = summary.Verified > 0 && summary.Unresolved+summary.Failed+summary.NotChecked > 0
	return summary
}

func (summary *pipelineVerificationSummary) add(status VerificationStatus) {
	switch status {
	case VerificationVerified:
		summary.Verified++
	case VerificationFailed:
		summary.Failed++
	case VerificationNotChecked:
		summary.NotChecked++
	default:
		summary.Unresolved++
	}
}

type pipelineVerificationCounts struct {
	verified   int
	unresolved int
	failed     int
	notChecked int
}

func (counts *pipelineVerificationCounts) add(status VerificationStatus) {
	switch status {
	case VerificationVerified:
		counts.verified++
	case VerificationFailed:
		counts.failed++
	case VerificationNotChecked:
		counts.notChecked++
	default:
		counts.unresolved++
	}
}

func pipelineVerificationSummaryStatus(
	verified int,
	unresolved int,
	failed int,
	notChecked int,
) verificationSummaryStatus {
	switch {
	case failed > 0:
		return verificationSummaryFailed
	case verified > 0 && unresolved+notChecked > 0:
		return verificationSummaryPartial
	case unresolved > 0:
		return verificationSummaryUnresolved
	case notChecked > 0:
		return verificationSummaryNotChecked
	case verified > 0:
		return verificationSummaryVerified
	default:
		return verificationSummaryNotChecked
	}
}
