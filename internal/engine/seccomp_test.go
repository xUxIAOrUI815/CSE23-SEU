package engine

import "testing"

func TestBuildBPFKnown(t *testing.T) {
	f, err := buildBPF([]string{"read", "write", "exit_group"})
	if err != nil {
		t.Fatalf("buildBPF: %v", err)
	}
	if len(f) == 0 {
		t.Fatal("expected non-empty filter")
	}
}

func TestBuildBPFUnknown(t *testing.T) {
	if _, err := buildBPF([]string{"not_a_syscall"}); err == nil {
		t.Fatal("expected error for unknown syscall")
	}
}

func TestSeccompBlocks(t *testing.T) {
	// 白名单只放 exit_group：execve 不在白名单内，目标无法启动
	code, err := Run(&Spec{SeccompAllow: []string{"exit_group"}, Command: []string{"/bin/true"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code == 0 {
		t.Error("expected non-zero exit when seccomp blocks execve")
	}
}
