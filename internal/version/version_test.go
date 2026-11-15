package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	vcs := func(rev, modified string) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "(devel)"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: rev},
				{Key: "vcs.time", Value: "2026-11-20T10:11:12Z"},
				{Key: "vcs.modified", Value: modified},
			},
		}
	}
	cases := []struct {
		name                string
		version, commit, dt string
		bi                  *debug.BuildInfo
		want                string
	}{
		{"link-time values win", "0.1.0", "abcdef1234567", "2026-01-02", vcs("zzzzzzzzzzzz", "false"), "pandora 0.1.0 (abcdef1, 2026-01-02, go1.27)"},
		{"go build uses vcs data", "", "", "", vcs("0123456789abcdef", "false"), "pandora dev (0123456, 2026-11-20, go1.27)"},
		{"dirty tree is flagged", "", "", "", vcs("0123456789abcdef", "true"), "pandora dev (0123456+dirty, 2026-11-20, go1.27)"},
		{"go install reports the module version", "", "", "", &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, "pandora 0.2.0 (go1.27)"},
		{"untagged build is dev, not a pseudo-version", "", "", "", &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261115135323-d03453e8e0c8+dirty"}}, "pandora dev (go1.27)"},
		{"nothing known", "", "", "", nil, "pandora dev (go1.27)"},
	}
	for _, c := range cases {
		if got := resolve(c.version, c.commit, c.dt, c.bi, "go1.27").String(); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
