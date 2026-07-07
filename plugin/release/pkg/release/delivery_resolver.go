package release

import (
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// DeliveryContract describes whether a configured delivery can be executed by
// this process. Remote deliveries may still be valid for dry-run planning.
type DeliveryContract struct {
	Type                   string
	SupportsLocalExecution bool
	Implemented            bool
}

// ResolveDelivery validates a delivery identifier and returns its execution
// contract without dispatching anything.
func ResolveDelivery(delivery string) (DeliveryContract, error) {
	switch releaseconfig.DeliveryType(delivery) {
	case releaseconfig.DeliveryLocal:
		return DeliveryContract{
			Type:                   string(releaseconfig.DeliveryLocal),
			SupportsLocalExecution: true,
			Implemented:            true,
		}, nil
	case releaseconfig.DeliveryGitHubActions:
		return DeliveryContract{
			Type:                   string(releaseconfig.DeliveryGitHubActions),
			SupportsLocalExecution: false,
			Implemented:            false,
		}, nil
	default:
		return DeliveryContract{}, fmt.Errorf("unknown delivery: %s", delivery)
	}
}
