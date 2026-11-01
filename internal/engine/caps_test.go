package engine

import (
	"io"
	"os"
	"strings"
	"testing"
)

func runCapture(t *testing.T, spec *Spec) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	code, err := Run(spec)
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout, os.Stderr = oldOut, oldErr
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, output=%s", code, out)
	}
	return string(out)
}

func extractCapField(t *testing.T, out, field string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, field+":") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	t.Fatalf("%s not found in output: %q", field, out)
	return ""
}

func TestDropCaps(t *testing.T) {
	// 默认（uid 已映射为 root，有 capability）：exec 后 CapEff 非零
	fullOut := runCapture(t, &Spec{Command: []string{"/bin/sh", "-c", "grep CapEff /proc/self/status"}})
	if cap := extractCapField(t, fullOut, "CapEff"); cap == "0000000000000000" {
		t.Fatalf("expected non-zero CapEff without DropCaps, got %q", cap)
	}

	// DropCaps=true：目标进程 exec 后 CapEff 也应为全零（降权真正生效）
	dropOut := runCapture(t, &Spec{DropCaps: true, Command: []string{"/bin/sh", "-c", "grep CapEff /proc/self/status"}})
	if cap := extractCapField(t, dropOut, "CapEff"); cap != "0000000000000000" {
		t.Errorf("expected zero CapEff with DropCaps, got %q", cap)
	}
}
