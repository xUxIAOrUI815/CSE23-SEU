package engine

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunNamespaceIsolation(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	code, err := Run(&Spec{Command: []string{"/bin/sh", "-c", "echo pid=$$; [ -f /etc/os-release ] && echo rootfs-ok"}})
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, output=%s", code, out)
	}
	s := string(out)
	if !strings.Contains(s, "pid=1") {
		t.Errorf("expected pid=1 in output, got %q", s)
	}
	if !strings.Contains(s, "rootfs-ok") {
		t.Errorf("expected rootfs-ok in output, got %q", s)
	}
}

func TestRunCommandNotFound(t *testing.T) {
	code, err := Run(&Spec{Command: []string{"/no/such/bin"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code == 0 {
		t.Error("expected non-zero exit code for missing command")
	}
}

func TestRunBareCommand(t *testing.T) {
	// 裸命令名应能通过 PATH 解析（syscall.Exec 本身不搜 PATH）
	out := runCapture(t, &Spec{Command: []string{"sh", "-c", "echo bare-ok"}})
	if !strings.Contains(out, "bare-ok") {
		t.Errorf("expected bare-ok in output, got %q", out)
	}
}
