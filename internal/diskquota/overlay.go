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
	upper := filepath.Join(instDir, "upper")
	work := filepath.Join(instDir, "work")
	for _, p := range []string{upper, work, rootfs} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return fmt.Errorf("diskquota: mkdir %s: %w", p, err)
		}
	}
	if err := os.Truncate(diskImg, diskBytes); err != nil {
		return fmt.Errorf("diskquota: truncate disk.img: %w", err)
	}
	if err := run("mkfs.ext4", "-F", "-q", diskImg); err != nil {
		return fmt.Errorf("diskquota: mkfs.ext4: %w", err)
	}
	if err := run("mount", "-o", "loop", diskImg, upper); err != nil {
		return fmt.Errorf("diskquota: mount upper: %w", err)
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := run("mount", "-t", "overlay", "overlay", "-o", opts, rootfs); err != nil {
		_ = run("umount", upper)
		return fmt.Errorf("diskquota: mount overlay: %w", err)
	}
	return nil
}

// Teardown unmounts overlay and loop mounts under instDir before RemoveAll.
func Teardown(instDir string) {
	_ = run("umount", filepath.Join(instDir, "rootfs"))
	_ = run("umount", filepath.Join(instDir, "upper"))
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", name, args, err, string(out))
	}
	return nil
}
