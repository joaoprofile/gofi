package config

import (
	"regexp"
	"testing"
)

// The default SDK source is a release, never a moving branch: the bundled Go
// docs describe one version and a new project must check out that version.
func TestDefaultSDKGoRefPinsARelease(t *testing.T) {
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(SDKGoVersion) {
		t.Fatalf("SDKGoVersion = %q, want vX.Y.Z", SDKGoVersion)
	}
	if want := SDKGoRepo + "@gofi/" + SDKGoVersion; DefaultSDKGoRef != want {
		t.Fatalf("DefaultSDKGoRef = %q, want %q", DefaultSDKGoRef, want)
	}
}
