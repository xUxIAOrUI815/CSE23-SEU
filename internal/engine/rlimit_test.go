package engine

import (
	"strings"
	"testing"
)

func TestMemoryLimit(t *testing.T) {
	// 设 32MB 地址空间上限（RLIMIT_AS），目标进程应能读到该上限
	out := runCapture(t, &Spec{
		Limits:  Limits{MemoryBytes: 32 * 1024 * 1024},
		Command: []string{"/bin/sh", "-c", "grep 'Max address space' /proc/self/limits"},
	})
	if !strings.Contains(out, "33554432") {
		t.Errorf("expected address space limit 33554432, got %q", out)
	}
}
