// Package version holds the build-stamped version of the Capper binaries.
//
// The values are set at build time via -ldflags, e.g.:
//
//	go build -ldflags "\
//	  -X capper/internal/version.Version=1.2.3 \
//	  -X capper/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X capper/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// When built without stamping (e.g. `go run`/`go test`), the defaults below apply.
package version

import (
	"fmt"
	"runtime"
)

// Stamped at build time via -ldflags -X. Defaults are used for unstamped builds.
var (
	Version   = "0.0.0-dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// APIVersion is the REST API contract major version. Bump only on a
// backward-incompatible API change; agents and clients negotiate against it.
const APIVersion = "v1"

// MinAgentVersion is the oldest node-agent version this control plane supports
// (N-1 policy). Skew older than this is warned about, not rejected.
var MinAgentVersion = "0.0.0"

// Info is the structured build identity returned by `capper version --json` and
// GET /api/v1/version.
type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"buildDate"`
	GoVersion  string `json:"goVersion"`
	APIVersion string `json:"apiVersion"`
	Platform   string `json:"platform"`
}

// Get returns the current build identity.
func Get() Info {
	return Info{
		Version:    Version,
		Commit:     Commit,
		BuildDate:  BuildDate,
		GoVersion:  runtime.Version(),
		APIVersion: APIVersion,
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String returns a one-line human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("capper %s (commit %s, built %s, %s, %s)",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}
