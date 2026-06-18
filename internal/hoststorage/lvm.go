package hoststorage

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// lvmExec runs an LVM/mkfs command. It is a package var so tests can stub it.
var lvmExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return exec.CommandContext(cctx, name, args...).CombinedOutput()
}

// lvmMu serializes LVM mutations so concurrent allocations never race lvcreate.
var lvmMu sync.Mutex

// lvmAvailable reports whether the LVM tools are installed.
func lvmAvailable() bool {
	_, err := exec.LookPath("lvs")
	return err == nil
}

// vgSizeBytes returns the total size of a volume group in bytes.
func vgSizeBytes(ctx context.Context, vg string) (int64, error) {
	out, err := lvmExec(ctx, "vgs", "--noheadings", "--units", "b", "--nosuffix", "-o", "vg_size", vg)
	if err != nil {
		return 0, fmt.Errorf("hoststorage: vgs %s: %w (%s)", vg, err, strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	n, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		return 0, fmt.Errorf("hoststorage: parse vg_size %q: %w", s, perr)
	}
	return n, nil
}

// lvCreate creates a logical volume of sizeBytes named lv in vg, then formats it
// ext4. It returns the device path. All LVM mutations are serialized.
func lvCreate(ctx context.Context, vg, lv string, sizeBytes int64) (string, error) {
	lvmMu.Lock()
	defer lvmMu.Unlock()
	out, err := lvmExec(ctx, "lvcreate", "-L", strconv.FormatInt(sizeBytes, 10)+"b", "-n", lv, vg)
	if err != nil {
		return "", fmt.Errorf("hoststorage: lvcreate %s/%s: %w (%s)", vg, lv, err, strings.TrimSpace(string(out)))
	}
	device := "/dev/" + vg + "/" + lv
	if out, err := lvmExec(ctx, "mkfs.ext4", "-F", "-q", device); err != nil {
		// Roll back the LV so a failed format does not leak capacity.
		_, _ = lvmExec(ctx, "lvremove", "-f", device)
		return "", fmt.Errorf("hoststorage: mkfs %s: %w (%s)", device, err, strings.TrimSpace(string(out)))
	}
	return device, nil
}

// lvRemove removes a logical volume device.
func lvRemove(ctx context.Context, device string) error {
	lvmMu.Lock()
	defer lvmMu.Unlock()
	out, err := lvmExec(ctx, "lvremove", "-f", device)
	if err != nil {
		return fmt.Errorf("hoststorage: lvremove %s: %w (%s)", device, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// sanitizeLVName makes a string safe for use as an LV name (alnum + '-'/'_').
func sanitizeLVName(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		out = "lv"
	}
	return out
}
