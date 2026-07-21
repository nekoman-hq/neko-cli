package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import "fmt"

// Legacy: tools retains the process-global V1 registry for compatibility only.
var tools = make(map[string]Tool)

// Register adds a legacy release tool to the process-global compatibility
// registry.
//
// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values
// instead.
func Register(t Tool) {
	tools[t.Name()] = t
}

// Get returns a legacy release tool from the process-global compatibility
// registry.
//
// Deprecated: use caller-owned V1Executor selection with
// HandleReleaseWithV1Executors instead.
func Get(name string) (Tool, error) {
	if t, ok := tools[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("unknown release system: %s", name)
}
