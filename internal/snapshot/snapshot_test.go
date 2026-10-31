package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestOptionString(t *testing.T) {
	s := optionString(Overlay{LowerDir: "/l", UpperDir: "/u", WorkDir: "/w"})
	if s != "lowerdir=/l,upperdir=/u,workdir=/w" {
		t.Errorf("optionString = %q", s)
	}
}

// TestOverlayFreeze 验证 overlayfs 的「冻结 + 丢弃写层」语义：
// 在 userns 内挂载（lower=冻结只读层），写入落到 upper（可丢弃写层），lower 不变。
// 挂载需要 CAP_SYS_ADMIN，这里用 util-linux 的 unshare 进入 userns（与引擎同款 rootless 方式）。
func TestOverlayFreeze(t *testing.T) {
	base := t.TempDir()
	upper := t.TempDir()
	work := t.TempDir()
	mnt := t.TempDir()
	if err := os.WriteFile(base+"/keep.txt", []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{base, upper, work, mnt} {
		_ = os.Chmod(d, 0o777)
	}

	script := fmt.Sprintf(
		"unshare --user --map-root-user --mount sh -c 'mount -t overlay overlay -o lowerdir=%s,upperdir=%s,workdir=%s %s && echo x > %s/new.txt && umount %s'",
		base, upper, work, mnt, mnt, mnt,
	)
	out, err := exec.Command("sh", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("overlay freeze failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(base + "/new.txt"); !os.IsNotExist(err) {
		t.Errorf("frozen lower layer should not contain new.txt")
	}
	if _, err := os.Stat(upper + "/new.txt"); err != nil {
		t.Errorf("writable upper layer should contain new.txt: %v", err)
	}
}
