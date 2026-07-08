package invoq

import (
	"runtime/debug"
	"testing"
)

func TestNormalizesModuleVersionForUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "tag", version: "v0.1.0", want: "0.1.0"},
		{name: "uppercase tag prefix", version: "V1.2.3", want: "1.2.3"},
		{
			name:    "pseudo version",
			version: "v0.0.0-20260708000000-abcdef123456",
			want:    "0.0.0-20260708000000-abcdef123456",
		},
		{name: "already normalized", version: "0.1.0", want: "0.1.0"},
		{name: "devel", version: "(devel)", want: unknownVersion},
		{name: "empty", version: " ", want: unknownVersion},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeModuleVersion(test.version); got != test.want {
				t.Fatalf("normalizeModuleVersion(%q) = %q, want %q", test.version, got, test.want)
			}
		})
	}
}

func TestReadsSDKVersionFromBuildInfoDependency(t *testing.T) {
	buildInfo := &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/merchant/app", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: modulePath, Version: "v0.1.0"},
		},
	}

	if got := sdkVersionFromBuildInfo(buildInfo); got != "0.1.0" {
		t.Fatalf("sdkVersionFromBuildInfo() = %q, want %q", got, "0.1.0")
	}
}

func TestReadsUnknownVersionForLocalReplacement(t *testing.T) {
	buildInfo := &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/merchant/app", Version: "(devel)"},
		Deps: []*debug.Module{
			{
				Path:    modulePath,
				Version: "v0.1.0",
				Replace: &debug.Module{
					Path: "../sdk-go",
				},
			},
		},
	}

	if got := sdkVersionFromBuildInfo(buildInfo); got != unknownVersion {
		t.Fatalf("sdkVersionFromBuildInfo() = %q, want %q", got, unknownVersion)
	}
}

func TestReadsUnknownVersionWhenSDKModuleIsMissing(t *testing.T) {
	buildInfo := &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/merchant/app", Version: "(devel)"},
	}

	if got := sdkVersionFromBuildInfo(buildInfo); got != unknownVersion {
		t.Fatalf("sdkVersionFromBuildInfo() = %q, want %q", got, unknownVersion)
	}
}
