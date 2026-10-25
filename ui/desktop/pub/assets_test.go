package pub

import (
	"testing"

	"github.com/vcaesar/codg/plugin/version"
)

func TestAppName(t *testing.T) {
	t.Parallel()
	if AppName != "Codg" {
		t.Fatalf("AppName = %q, want %q", AppName, "Codg")
	}
}

func TestAppVersionDefault(t *testing.T) {
	t.Parallel()
	if AppVersion != version.Version {
		t.Fatalf("AppVersion = %q, want plugin/version %q", AppVersion, version.Version)
	}
}
