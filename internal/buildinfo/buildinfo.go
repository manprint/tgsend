package buildinfo

// Info contains the build metadata exposed by the CLI.
type Info struct {
	Version string
	Commit  string
	Date    string
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// TestEndpointEnabled is false in every normal build. E2E builds enable it
	// with -ldflags and may then use a loopback-only Telegram endpoint.
	TestEndpointEnabled = "false"
)

// Current returns the build metadata. Release tooling overrides the package
// variables with -ldflags -X; development builds use stable defaults.
func Current() Info {
	return Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}
