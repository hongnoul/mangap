package version

import (
	"regexp"
	"testing"
)

// TestLatestVersion hits the GitHub releases API, so it skips when the
// network or the API is unavailable (offline, rate limit, no release yet)
// and only checks the format of what comes back.
func TestLatestVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	version, err := Latest()
	if err != nil {
		t.Skipf("GitHub releases API unavailable: %v", err)
	}
	semver := regexp.MustCompile(`^v?(\d+)(\.\d+){0,2}(-\w+)?$`)
	if !semver.MatchString(version) {
		t.Fatalf("latest version %q is not semver", version)
	}
}
