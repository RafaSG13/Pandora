// Package version reports which build of Pandora is running.
//
// Release builds set Version, Commit and Date with -ldflags -X. Builds without
// them (go build, go install) fall back to the VCS data Go embeds in the
// binary, and report "dev" when even that is absent.
package version

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set at link time by the release build; see the Makefile and .goreleaser.yaml.
var (
	Version = ""
	Commit  = ""
	Date    = ""
)

// Info is the resolved build information.
type Info struct {
	Version string // without a leading "v", "dev" when unknown
	Commit  string // abbreviated to 7 characters, "+dirty" when the tree was modified
	Date    string
	Go      string
}

// Get returns the build information.
func Get() Info {
	bi, _ := debug.ReadBuildInfo()
	return resolve(Version, Commit, Date, bi, runtime.Version())
}

// String is the one-line form printed by `pandora --version`.
func String() string { return Get().String() }

func (i Info) String() string {
	var details []string
	for _, d := range []string{i.Commit, i.Date, i.Go} {
		if d != "" {
			details = append(details, d)
		}
	}
	return fmt.Sprintf("pandora %s (%s)", i.Version, strings.Join(details, ", "))
}

// pseudoVersion matches the version Go stamps on untagged builds, such as
// v0.0.0-20261115135323-d03453e8e0c8+dirty.
var pseudoVersion = regexp.MustCompile(`-\d{14}-[0-9a-f]{12}(\+dirty)?$`)

// resolve prefers link-time values and fills the gaps from bi.
func resolve(version, commit, date string, bi *debug.BuildInfo, goVersion string) Info {
	info := Info{Version: version, Commit: commit, Date: date, Go: goVersion}
	if bi != nil {
		if info.Version == "" && bi.Main.Version != "(devel)" && !pseudoVersion.MatchString(bi.Main.Version) {
			info.Version = bi.Main.Version
		}
		var rev, modified string
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				if info.Date == "" {
					info.Date = s.Value
				}
			case "vcs.modified":
				modified = s.Value
			}
		}
		if info.Commit == "" && rev != "" {
			info.Commit = rev
			if modified == "true" {
				info.Commit += "+dirty"
			}
		}
	}
	info.Version = strings.TrimPrefix(info.Version, "v")
	if info.Version == "" {
		info.Version = "dev"
	}
	if len(info.Commit) > 7 && !strings.Contains(info.Commit[:7], "+") {
		suffix := ""
		if strings.HasSuffix(info.Commit, "+dirty") {
			suffix = "+dirty"
		}
		info.Commit = info.Commit[:7] + suffix
	}
	if len(info.Date) > 10 {
		info.Date = info.Date[:10]
	}
	return info
}
