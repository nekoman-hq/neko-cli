package goreleaser

import "testing"

func TestClassifyArguments(t *testing.T) {
	tests := []struct {
		name string
		args string
		want Invocation
	}{
		{name: "check", args: "check --config .goreleaser.yaml", want: Invocation{Command: "check", ConfigReference: ".goreleaser.yaml"}},
		{name: "release", args: "release --clean --config=.goreleaser.yml", want: Invocation{Command: "release", ConfigReference: ".goreleaser.yml", RealPublication: true}},
		{name: "snapshot", args: "release --snapshot=true --config .goreleaser.yml", want: Invocation{Command: "release", ConfigReference: ".goreleaser.yml", Snapshot: true}},
		{name: "skip equals", args: "release --skip=validate,publish", want: Invocation{Command: "release", SkipPublication: true}},
		{name: "skip separate", args: "release --skip 'validate,publish'", want: Invocation{Command: "release", SkipPublication: true}},
		{name: "workflow expression", args: `release --config "${{ env.GORELEASER_CONFIG }}"`, want: Invocation{Command: "release", ConfigReference: "${{env.GORELEASER_CONFIG}}", RealPublication: true}},
		{name: "quoted raw command preserves non-publication", args: `'release' --config .goreleaser.yml`, want: Invocation{Command: "release", ConfigReference: ".goreleaser.yml"}},
		{name: "quoted comma item preserves raw publication", args: `release --skip=validate,"publish"`, want: Invocation{Command: "release", SkipPublication: true, RealPublication: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyArguments(tt.args); got != tt.want {
				t.Fatalf("ClassifyArguments(%q) = %#v, want %#v", tt.args, got, tt.want)
			}
		})
	}
}
