package version

// Set via -ldflags "-X github.com/gvault/gvault/internal/version.Version=..."
var (
	Version   = "0.1.0-beta.1"
	Commit    = "dev"
	BuildDate = "unknown"
)

func Info() map[string]string {
	return map[string]string{
		"product":   "gvault",
		"version":   Version,
		"commit":    Commit,
		"buildDate": BuildDate,
		"channel":   "beta",
	}
}
