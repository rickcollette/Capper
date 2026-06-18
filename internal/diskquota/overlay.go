package diskquota

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SetupOverlay replaces instDir/rootfs with an overlay whose upper layer is a
// size-capped ext4 loop device. When diskBytes <= 0 the extracted rootfs is
// left unchanged. Requires root for mount/mkfs; failures are returned to the
// caller so launches fail loudly instead of silently skipping the limit.
func SetupOverlay(instDir string, diskBytes int64) error {
	if diskBytes <= 0 {
		return nil
	}
	rootfs := filepath.Join(instDir, "rootfs")
	lower := filepath.Join(instDir, "lower")
	if err := os.Rename(rootfs, lower); err != nil {
		return fmt.Errorf("diskquota: rename rootfs: %w", err)
	}
	diskImg := filepath.Join(instDir, "disk.img")
	cow := filepath.Join(instDir, "cow")
	upper := filepath.Join(cow, "upper")
	work := filepath.Join(cow, "work")
	for _, p := range []string{cow, rootfs} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return fmt.Errorf("diskquota: mkdir %s: %w", p, err)
		}
	}
	f, err := os.OpenFile(diskImg, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("diskquota: create disk.img: %w", err)
	}
	if err := f.Truncate(diskBytes); err != nil {
		_ = f.Close()
		return fmt.Errorf("diskquota: truncate disk.img: %w", err)
	}
	_ = f.Close()
	if err := run("mkfs.ext4", "-F", "-q", diskImg); err != nil {
		return fmt.Errorf("diskquota: mkfs.ext4: %w", err)
	}
	if err := run("mount", "-o", "loop", diskImg, cow); err != nil {
		return fmt.Errorf("diskquota: mount cow: %w", err)
	}
	for _, p := range []string{upper, work} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			_ = run("umount", cow)
			return fmt.Errorf("diskquota: mkdir %s: %w", p, err)
		}
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := run("mount", "-t", "overlay", "overlay", "-o", opts, rootfs); err != nil {
		_ = run("umount", cow)
		return fmt.Errorf("diskquota: mount overlay: %w", err)
	}
	return nil
}

// Teardown unmounts overlay and loop mounts under instDir before RemoveAll.
func Teardown(instDir string) {
	_ = run("umount", filepath.Join(instDir, "rootfs"))
	_ = run("umount", filepath.Join(instDir, "cow"))
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", name, args, err, string(out))
	}
	return nil
}
