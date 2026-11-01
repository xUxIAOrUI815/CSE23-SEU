package engine

import (
	"os"
	"strings"
	"testing"

	"cryolab/internal/snapshot"
)

func TestOverlayMount(t *testing.T) {
	base := t.TempDir()
	upper := t.TempDir()
	work := t.TempDir()
	mnt := t.TempDir()
	if err := os.WriteFile(base+"/keep.txt", []byte("frozen"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 目标进程在 overlay 挂载点里读 lower 的文件、写新文件（应落到 upper）
	out := runCapture(t, &Spec{
		Overlay: &snapshot.Overlay{LowerDir: base, UpperDir: upper, WorkDir: work, MountPoint: mnt},
		Command: []string{"/bin/sh", "-c", "cat " + mnt + "/keep.txt && echo new > " + mnt + "/new.txt"},
	})
	if !strings.Contains(out, "frozen") {
		t.Errorf("expected frozen content from lower layer, got %q", out)
	}

	// 写进了 upper（可丢弃写层），冻结的 lower 不变
	if _, err := os.Stat(base + "/new.txt"); !os.IsNotExist(err) {
		t.Errorf("frozen lower layer should not contain new.txt")
	}
	if _, err := os.Stat(upper + "/new.txt"); err != nil {
		t.Errorf("writable upper layer should contain new.txt: %v", err)
	}
}
