//go:build unix

package pub

import (
	"os/exec"
	"testing"
)

func TestCodgBinaryName(t *testing.T) {
	t.Parallel()
	// Unix build (see build tag): never the Windows name.
	if got := CodgBinaryName(); got != "codg" {
		t.Fatalf("CodgBinaryName() = %q, want %q", got, "codg")
	}
}

func TestConfigureSysProcAttr(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("true")
	ConfigureSysProcAttr(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr = %+v, want Setpgid=true", cmd.SysProcAttr)
	}
}

func TestSignalProcessTreeNilProcess(t *testing.T) {
	t.Parallel()
	// Signalling an unstarted command must be a safe no-op.
	cmd := exec.Command("true")
	if err := InterruptProcessTree(cmd); err != nil {
		t.Fatalf("InterruptProcessTree(unstarted) = %v, want nil", err)
	}
	if err := KillProcessTree(cmd); err != nil {
		t.Fatalf("KillProcessTree(unstarted) = %v, want nil", err)
	}
}

func TestCodgPidsOnPortEmpty(t *testing.T) {
	t.Parallel()
	// Nothing named codg listens on this reserved port -> empty, no error.
	pids, err := CodgPidsOnPort(1) // tcpmux; never a codg backend
	if err != nil {
		t.Fatalf("CodgPidsOnPort: %v", err)
	}
	if len(pids) != 0 {
		t.Fatalf("CodgPidsOnPort = %v, want empty", pids)
	}
}

func TestIsCodgProcessSelf(t *testing.T) {
	t.Parallel()
	// The test binary is not named codg.
	if isCodgProcess(1) {
		t.Fatal("isCodgProcess(1) = true, want false")
	}
}
