package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"capper/internal/types"
)

// TestEnvListCleanBaseline verifies that envList never inherits host
// environment variables and always starts with a minimal PATH.
func TestEnvListCleanBaseline(t *testing.T) {
	// Poison the host environment with a value that must NOT appear in output.
	t.Setenv("LD_PRELOAD", "/evil.so")
	t.Setenv("SECRET_TOKEN", "supersecret")

	got := envList(map[string]string{"FOO": "bar"})

	for _, entry := range got {
		if strings.HasPrefix(entry, "LD_PRELOAD=") {
			t.Errorf("LD_PRELOAD leaked into capsule env: %s", entry)
		}
		if strings.HasPrefix(entry, "SECRET_TOKEN=") {
			t.Errorf("SECRET_TOKEN leaked into capsule env: %s", entry)
		}
	}

	hasPath := false
	hasFoo := false
	for _, entry := range got {
		if strings.HasPrefix(entry, "PATH=") {
			hasPath = true
		}
		if entry == "FOO=bar" {
			hasFoo = true
		}
	}
	if !hasPath {
		t.Error("envList did not include a PATH entry")
	}
	if !hasFoo {
		t.Error("envList did not include configured FOO=bar")
	}
}

// TestEnvListEmpty verifies that an empty env map still produces at least the
// PATH baseline.
func TestEnvListEmpty(t *testing.T) {
	got := envList(nil)
	if len(got) == 0 {
		t.Fatal("expected at least one entry (PATH baseline)")
	}
	for _, entry := range got {
		if strings.HasPrefix(entry, "PATH=") {
			return
		}
	}
	t.Error("envList did not include PATH baseline for empty map")
}

// TestEnvListMultipleVars verifies all configured vars are present exactly once.
func TestEnvListMultipleVars(t *testing.T) {
	env := map[string]string{
		"HOME": "/root",
		"USER": "capsule",
		"PORT": "8080",
	}
	got := envList(env)
	counts := map[string]int{}
	for _, entry := range got {
		k := strings.SplitN(entry, "=", 2)[0]
		counts[k]++
	}
	for k := range env {
		if counts[k] != 1 {
			t.Errorf("expected exactly 1 entry for %s, got %d", k, counts[k])
		}
	}
}

// TestAliveCurrentPID verifies that the current process reports as alive.
func TestAliveCurrentPID(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Error("Alive(os.Getpid()) returned false; current process should be alive")
	}
}

// TestAliveDeadPID verifies that an impossible PID is not alive.
func TestAliveDeadPID(t *testing.T) {
	if Alive(999999999) {
		t.Error("Alive(999999999) returned true; no such process should exist")
	}
}

// TestAliveZero verifies that PID 0 is never considered alive.
func TestAliveZero(t *testing.T) {
	if Alive(0) {
		t.Error("Alive(0) returned true; zero PID should never be alive")
	}
}

// TestApplyResourceLimitsNoOp verifies that all-zero limits don't error.
func TestApplyResourceLimitsNoOp(t *testing.T) {
	limits := types.ResourceLimits{}
	if err := applyResourceLimits(limits, true); err != nil {
		t.Errorf("applyResourceLimits with zero limits: %v", err)
	}
	if err := applyResourceLimits(limits, false); err != nil {
		t.Errorf("applyResourceLimits(enforcePids=false) with zero limits: %v", err)
	}
}

// TestWriteProcOverrides verifies that the generated cpuinfo/meminfo files
// reflect the capsule's resource limits and do NOT expose host data.
func TestWriteProcOverrides(t *testing.T) {
	dir := t.TempDir()
	resources := types.ResourceLimits{MemoryBytes: 256 * 1024 * 1024} // 256 MiB
	if err := WriteProcOverrides(dir, resources); err != nil {
		t.Fatalf("WriteProcOverrides: %v", err)
	}

	cpuinfo, err := os.ReadFile(dir + "/proc-overrides/cpuinfo")
	if err != nil {
		t.Fatalf("cpuinfo not written: %v", err)
	}
	if !strings.Contains(string(cpuinfo), "processor\t: 0") {
		t.Error("cpuinfo missing processor entry")
	}
	// Must not expose more than 1 vCPU (host has 4).
	if strings.Contains(string(cpuinfo), "processor\t: 1") {
		t.Error("cpuinfo exposes more than 1 vCPU")
	}

	meminfo, err := os.ReadFile(dir + "/proc-overrides/meminfo")
	if err != nil {
		t.Fatalf("meminfo not written: %v", err)
	}
	// 256 MiB = 262144 kB — verify MemTotal matches the allocation exactly.
	if !strings.Contains(string(meminfo), "MemTotal:") {
		t.Error("meminfo missing MemTotal")
	}
	if !strings.Contains(string(meminfo), "262144") {
		t.Errorf("meminfo MemTotal should be 262144 kB for 256 MiB; got:\n%s", meminfo)
	}
	// Host has ~31 GiB = ~32505856 kB. Make sure that value is absent.
	if strings.Contains(string(meminfo), "32505856") || strings.Contains(string(meminfo), "31457280") {
		t.Error("meminfo exposes host total memory")
	}
}

// TestWriteProcOverridesDefault verifies that zero MemoryBytes uses a capped default.
func TestWriteProcOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	if err := WriteProcOverrides(dir, types.ResourceLimits{}); err != nil {
		t.Fatalf("WriteProcOverrides: %v", err)
	}
	meminfo, err := os.ReadFile(dir + "/proc-overrides/meminfo")
	if err != nil {
		t.Fatalf("meminfo not written: %v", err)
	}
	// Default is 512 MiB → 524288 kB, far below 31 GiB.
	if strings.Contains(string(meminfo), "31") {
		t.Error("default meminfo exposes host total memory")
	}
}

// TestAppendProcOverridesIncludesFiles verifies bind-mount args are added when
// the proc-overrides directory exists.
func TestAppendProcOverridesIncludesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := WriteProcOverrides(dir, types.ResourceLimits{MemoryBytes: 128 * 1024 * 1024}); err != nil {
		t.Fatalf("WriteProcOverrides: %v", err)
	}
	args := appendProcOverrides(nil, dir)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ro-bind") {
		t.Error("appendProcOverrides did not add --ro-bind")
	}
	if !strings.Contains(joined, "/proc/cpuinfo") {
		t.Error("appendProcOverrides did not bind /proc/cpuinfo")
	}
	if !strings.Contains(joined, "/proc/meminfo") {
		t.Error("appendProcOverrides did not bind /proc/meminfo")
	}
}

// TestAppendProcOverridesEmpty verifies no args are added when the directory is absent.
func TestAppendProcOverridesEmpty(t *testing.T) {
	args := appendProcOverrides(nil, t.TempDir())
	if len(args) != 0 {
		t.Errorf("expected no args for missing proc-overrides dir, got %v", args)
	}
}

// TestBwrapShellCmdHasUnshareNet verifies that the shell PTY command always
// includes --unshare-net when no named netns is provided.
func TestBwrapShellCmdHasUnshareNet(t *testing.T) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bwrap not available")
	}
	dir := t.TempDir()
	_ = os.MkdirAll(dir+"/rootfs", 0o755)
	cmd := buildBwrapShellCmd(bwrap, dir+"/rootfs", "/bin/sh", "", types.UserConfig{UID: 1000, GID: 1000})
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "--unshare-net") {
		t.Errorf("bwrap shell cmd missing --unshare-net; args: %s", joined)
	}
}

// TestRunLimitedSpecUnknownMode verifies that an unknown mode falls through to
// the chroot path and returns an error when not running as root.
func TestRunLimitedSpecUnknownMode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chroot path would succeed")
	}
	dir := t.TempDir()
	spec := launchSpec{
		Mode:    "chroot",
		InstDir: dir,
		RootFS:  dir,
		Manifest: types.CapsuleManifest{
			Entrypoint: []string{"/bin/sh"},
			WorkingDir: "/",
		},
	}
	err := runLimitedSpec(spec)
	if err == nil {
		t.Fatal("expected error from chroot as non-root, got nil")
	}
	if !strings.Contains(err.Error(), "chroot requires elevated") {
		t.Errorf("unexpected error: %v", err)
	}
}
