package pub

import "testing"

func TestAppName(t *testing.T) {
	t.Parallel()
	if AppName != "Codg" {
		t.Fatalf("AppName = %q, want %q", AppName, "Codg")
	}
}
