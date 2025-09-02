package version

import (
	"fmt"
	"runtime"
)

// Build-time variables injected via -ldflags
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// String returns the full version string
func String() string {
	return fmt.Sprintf("ask %s\ncommit: %s\nbuilt: %s\ngo: %s",
		Version, GitCommit, BuildDate, runtime.Version())
}

// Short returns just the version number
func Short() string {
	return Version
}
