package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	if code := Run([]string{"nope"}, io.Discard, &stderr); code == 0 {
		t.Error("expected non-zero exit for unknown command")
	}
	if !strings.Contains(stderr.String(), "未知") {
		t.Errorf("stderr should contain usage, got %q", stderr.String())
	}
}

func TestDiffMissingArgs(t *testing.T) {
	var stderr bytes.Buffer
	if code := Run([]string{"diff", "onlyOne"}, io.Discard, &stderr); code == 0 {
		t.Error("expected non-zero exit for missing diff arg")
	}
}

func TestRunUnknownSnapshot(t *testing.T) {
	os.Setenv("CRYO_STORE", t.TempDir())
	defer os.Unsetenv("CRYO_STORE")
	var stderr bytes.Buffer
	if code := Run([]string{"run", "nope", "--", "true"}, io.Discard, &stderr); code == 0 {
		t.Error("expected non-zero exit for unknown snapshot")
	}
	if !strings.Contains(stderr.String(), "nope") {
		t.Errorf("stderr should mention the snapshot name, got %q", stderr.String())
	}
}

func TestSnapshotAndDiff(t *testing.T) {
	os.Setenv("CRYO_STORE", t.TempDir())
	defer os.Unsetenv("CRYO_STORE")

	dir := t.TempDir()
	m1 := filepath.Join(dir, "m1.json")
	m2 := filepath.Join(dir, "m2.json")
	_ = os.WriteFile(m1, []byte(`{"name":"v1","version":"1","command":["echo","a"],"prompt":"A"}`), 0o644)
	_ = os.WriteFile(m2, []byte(`{"name":"v2","version":"2","command":["echo","b"],"prompt":"B"}`), 0o644)

	if code := Run([]string{"snapshot", "v1", m1}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("snapshot v1 exit %d", code)
	}
	if code := Run([]string{"snapshot", "v2", m2}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("snapshot v2 exit %d", code)
	}

	var out bytes.Buffer
	if code := Run([]string{"diff", "v1", "v2"}, &out, io.Discard); code != 0 {
		t.Fatalf("diff exit %d", code)
	}
	if !strings.Contains(out.String(), "prompt") {
		t.Errorf("diff output should mention prompt change, got %q", out.String())
	}
}
