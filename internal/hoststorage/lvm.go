package hoststorage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// lvmExec runs an LVM/mkfs command. It is a package var so tests can stub it.
// LVM_SUPPRESS_FD_WARNINGS silences the "File descriptor N leaked on vgs
// invocation" notices LVM prints when the parent process holds open fds (the
// control daemon holds many sockets); without it those notices pollute the
// captured output.
var lvmExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	cmd.Env = append(os.Environ(), "LVM_SUPPRESS_FD_WARNINGS=1")
	return cmd.CombinedOutput()
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
	// vg_size is a single integer; take the last whitespace-delimited token so any
	// stray warning lines that still reach the output can't corrupt the value.
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0, fmt.Errorf("hoststorage: vgs %s returned no size", vg)
	}
	s := fields[len(fields)-1]
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
	// -y answers prompts (non-interactive exec has no tty), and -Wy auto-wipes any
	// leftover filesystem signature on the reused extents — otherwise lvcreate
	// prompts "Wipe it? [y/n]" and aborts.
	out, err := lvmExec(ctx, "lvcreate", "-y", "-Wy",
		"-L", strconv.FormatInt(sizeBytes, 10)+"b", "-n", lv, vg)
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
