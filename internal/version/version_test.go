package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	vcs := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "587c1271f0e4a0f4b3d0c9e1a2b3c4d5e6f7a8b9"},
		{Key: "vcs.time", Value: "2026-09-26T10:00:00Z"},
		{Key: "vcs.modified", Value: "false"},
	}

	tests := []struct {
		name        string
		stamped     Info
		mainVersion string
		settings    []debug.BuildSetting
		want        Info
	}{
		{
			name:        "linker-stamped values win over build info",
			stamped:     Info{Version: "v1.0.0", Commit: "abc", Date: "2026-01-01"},
			mainVersion: "v0.9.0",
			settings:    vcs,
			want:        Info{Version: "v1.0.0", Commit: "abc", Date: "2026-01-01"},
		},
		{
			name:        "go install records the module version",
			stamped:     Info{Version: "dev"},
			mainVersion: "v1.0.0",
			want:        Info{Version: "v1.0.0"},
		},
		{
			name:        "plain go build keeps dev and picks up vcs info",
			stamped:     Info{Version: "dev"},
			mainVersion: "(devel)",
			settings:    vcs,
			want: Info{
				Version: "dev",
				Commit:  "587c1271f0e4a0f4b3d0c9e1a2b3c4d5e6f7a8b9",
				Date:    "2026-09-26T10:00:00Z",
			},
		},
		{
			name:        "untagged commit's pseudo-version stays dev",
			stamped:     Info{Version: "dev"},
			mainVersion: "v0.0.0-20260819145859-587c127e510b",
			want:        Info{Version: "dev"},
		},
		{
			name:        "dirty pseudo-version stays dev",
			stamped:     Info{Version: "dev"},
			mainVersion: "v1.0.1-0.20260819145859-587c127e510b+dirty",
			want:        Info{Version: "dev"},
		},
		{
			name:        "build at a tag with local edits keeps the tag",
			stamped:     Info{Version: "dev"},
			mainVersion: "v1.0.0+dirty",
			want:        Info{Version: "v1.0.0"},
		},
		{
			name:        "uncommitted changes are flagged",
			stamped:     Info{Version: "dev"},
			mainVersion: "(devel)",
			settings:    []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}},
			want:        Info{Version: "dev", Dirty: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bi := &debug.BuildInfo{Settings: tt.settings}
			bi.Main.Version = tt.mainVersion

			got := resolve(tt.stamped, bi)
			if got != tt.want {
				t.Errorf("resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInfoString(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{"version only", Info{Version: "dev"}, "dev"},
		{
			"full release build",
			Info{Version: "v1.0.0", Commit: "587c1271f0e4", Date: "2026-09-26T10:00:00Z"},
			"v1.0.0 (commit 587c127, built 2026-09-26T10:00:00Z)",
		},
		{"dirty build", Info{Version: "dev", Commit: "587c127", Dirty: true}, "dev (commit 587c127, modified)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
