package contextvalidation

import (
	"fmt"
	"strings"
)

type releaseContextGitAdapter struct {
	runner contextGitCommandRunner
}

func (adapter releaseContextGitAdapter) ObjectFormat(root string) (GitObjectFormat, error) {
	output, err := adapter.run(root, "rev-parse", "--show-object-format")
	if err != nil {
		return "", err
	}
	return GitObjectFormat(strings.TrimSpace(output)), nil
}

func (adapter releaseContextGitAdapter) ObjectType(root, objectID string) (string, error) {
	output, err := adapter.run(root, "cat-file", "-t", objectID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (adapter releaseContextGitAdapter) HeadCommit(root string) (string, error) {
	output, err := adapter.run(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (adapter releaseContextGitAdapter) TagExists(root, tag string) (bool, error) {
	_, err := adapter.run(root, "show-ref", "--verify", "--quiet", "refs/tags/"+tag)
	if err == nil {
		return true, nil
	}
	if isContextGitNotFound(err) {
		return false, nil
	}
	return false, err
}

func (adapter releaseContextGitAdapter) TagCommit(root, tag string) (string, error) {
	output, err := adapter.run(root, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (adapter releaseContextGitAdapter) run(root string, args ...string) (string, error) {
	if adapter.runner == nil {
		return "", fmt.Errorf("release context Git reader is missing")
	}
	return adapter.runner.Run(root, args...)
}
