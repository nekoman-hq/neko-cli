// Package localaction resolves repository-local composite GitHub Actions into
// the effective steps a workflow really runs.
//
// Resolution is read-only and repository-confined. It parses `action.yml` or
// `action.yaml` files below one repository root, expands `runs.using:
// composite` steps, inherits the invoking step's environment, and substitutes
// declared action inputs with the values supplied at the invocation. It never
// executes shell or Git commands, never resolves tokens or runtime
// expressions, and never follows remote, Docker, or absolute references.
package localaction
