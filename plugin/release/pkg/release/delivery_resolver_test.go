package release

import "testing"

func TestResolveDelivery(t *testing.T) {
	tests := []struct {
		name                   string
		delivery               string
		supportsLocalExecution bool
		implemented            bool
	}{
		{
			name:                   "local",
			delivery:               "local",
			supportsLocalExecution: false,
			implemented:            false,
		},
		{
			name:                   "github actions",
			delivery:               "github-actions",
			supportsLocalExecution: false,
			implemented:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contract, err := ResolveDelivery(tt.delivery)
			if err != nil {
				t.Fatalf("ResolveDelivery(%q): %v", tt.delivery, err)
			}
			if contract.Type != tt.delivery {
				t.Fatalf("expected type %q, got %#v", tt.delivery, contract)
			}
			if contract.SupportsLocalExecution != tt.supportsLocalExecution || contract.Implemented != tt.implemented {
				t.Fatalf("unexpected contract: %#v", contract)
			}
		})
	}
}

func TestResolveDeliveryRejectsUnknown(t *testing.T) {
	if _, err := ResolveDelivery("manual"); err == nil {
		t.Fatal("expected unknown delivery error")
	}
}
