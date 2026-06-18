package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"capper/internal/network"
	"capper/internal/oci"
	"capper/internal/types"
)

const (
	ModeAuto   = "auto"
	ModeBwrap  = "bwrap"
	ModeChroot = "chroot"
	ModeCrun   = "crun"
	ModeRunc   = "runc"
)

type Runner struct {
	Mode string
}

type launchSpec struct {
	Mode        string                `json:"mode"`
	ContainerID string                `json:"containerID,omitempty"`
	InstDir     string                `json:"instDir"`
	RootFS      string                `json:"rootfs"`
	Manifest    types.CapsuleManifest `json:"manifest"`
	NetNS       string                `json:"netNS,omitempty"` // named network namespace; "" → unshare-net
}

// StartOptions controls optional launch parameters.
type StartOptions struct {
	NetNS string // named network namespace to join; "" means unshare-net
}

func (r Runner) Start(instanceID, instDir string, manifest types.CapsuleManifest, opts StartOptions) (int, error) {
	rootfs := filepath.Join(instDir, "rootfs")
	entry := manifest.Entrypoint[0]
	if _, err := os.Stat(filepath.Join(rootfs, entry)); err != nil {
		return 0, fmt.Errorf("entrypoint not found inside rootfs: %s", entry)
	}
	specPath := filepath.Join(instDir, "launch.json")
	if err := writeLaunchSpec(specPath, launchSpec{
		Mode:        r.mode(),
		ContainerID: instanceID,
		InstDir:     instDir,
		RootFS:      rootfs,
		Manifest:    manifest,
		NetNS:       opts.NetNS,
	}); err != nil {
		return 0, err
	}
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(self, "__run-limited", specPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func RunLimitedLauncher(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var spec launchSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return err
	}
	if err := runLimitedSpec(spec); err != nil {
		writeStartupError(spec.InstDir, err)
		return err
	}
	return nil
}

func runLimitedSpec(spec launchSpec) error {
	mode := spec.Mode
	if mode == ModeCrun || mode == ModeRunc {
		bin, err := exec.LookPath(mode)
		if err != nil {
			return fmt.Errorf("%s runtime requested but not found: %w", mode, err)
		}
		return execOCI(bin, spec.ContainerID, spec.InstDir, spec.RootFS, spec.Manifest)
	}
	if mode == ModeBwrap || mode == ModeAuto {
		bwrap, err := exec.LookPath("bwrap")
		if err == nil {
			if err := applyResourceLimits(spec.Manifest.Resources, false); err != nil {
				return err
			}
			return execBwrap(bwrap, spec.InstDir, spec.RootFS, spec.Manifest, spec.NetNS)
		}
		if mode == ModeBwrap {
			return fmt.Errorf("bubblewrap runtime requested but bwrap was not found")
		}
	}
	if err := applyResourceLimits(spec.Manifest.Resources, true); err != nil {
		return err
	}
	return execChroot(spec.InstDir, spec.RootFS, spec.Manifest)
}

func execOCI(runtime, containerID, instDir, rootfs string, manifest types.CapsuleManifest) error {
	_ = rootfs // rootfs lives at instDir/rootfs; config.json uses relative "rootfs"
	if _, err := oci.Generate(instDir, manifest); err != nil {
		return fmt.Errorf("generate OCI bundle: %w", err)
	}
	stdout, stderr, err := openLogs(instDir)
	if err != nil {
		return err
	}
	args := []string{"run", "--bundle", instDir, containerID}
	return execWithLogs(runtime, args, stdout, stderr, os.Environ())
}

func execBwrap(bwrap, instDir, rootfs string, manifest types.CapsuleManifest, netNS string) error {
	stdout, stderr, err := openLogs(instDir)
	if err != nil {
		return err
	}
	args := []string{
		"--clearenv",
		"--unshare-user",
		"--uid", strconv.Itoa(manifest.User.UID),
		"--gid", strconv.Itoa(manifest.User.GID),
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
	}
	if manifest.Hostname != "" {
		args = append(args, "--hostname", manifest.Hostname)
	}
	// Network namespace: if a pre-configured netns is provided, run bwrap inside
	// it via `ip netns exec`; otherwise unshare to create a fresh empty namespace.
	if netNS == "" {
		args = append(args, "--unshare-net")
	}
	args = append(args,
		"--bind", rootfs, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--chdir", manifest.WorkingDir,
	)
	// Bind-mount per-instance proc overrides so guests see their own resource view.
	args = appendProcOverrides(args, instDir)
	for _, m := range manifest.Mounts {
		switch m.Type {
		case "tmpfs":
			args = append(args, "--tmpfs", m.Target)
		default:
			if m.ReadOnly {
				args = append(args, "--ro-bind", m.Source, m.Target)
			} else {
				args = append(args, "--bind", m.Source, m.Target)
			}
		}
	}
	for k, v := range manifest.Env {
		args = append(args, "--setenv", k, v)
	}
	// Bind-mount the per-instance metadata token read-only if a host path is set.
	if manifest.MetadataToken != "" {
		args = append(args, "--ro-bind-try", manifest.MetadataToken, "/run/capper/metadata-token")
	}
	if manifest.UseCapinit {
		// Wrap: /sbin/capinit -- <original-entrypoint> [args...]
		args = append(args, "--", "/sbin/capinit", "--")
		args = append(args, manifest.Entrypoint...)
		args = append(args, manifest.Args...)
	} else {
		args = append(args, "--", manifest.Entrypoint[0])
		args = append(args, manifest.Entrypoint[1:]...)
		args = append(args, manifest.Args...)
	}

	if netNS != "" {
		// Enter the named network namespace directly in Go before exec'ing bwrap.
		// setns(CLONE_NEWNET) requires only CAP_NET_ADMIN when the ns is owned by
		// the initial user namespace, which is the case for all capper-created namespaces.
		ns, err := netns.GetFromPath(network.NetNSPath(netNS))
		if err != nil {
			stdout.Close()
			stderr.Close()
			return fmt.Errorf("netns: open %s: %w", netNS, err)
		}
		// Lock the OS thread — setns is thread-local. Since execWithLogs calls
		// syscall.Exec (replacing the process image), the lock is never released,
		// which is intentional.
		runtime.LockOSThread()
		if err := unix.Setns(int(ns), unix.CLONE_NEWNET); err != nil {
			ns.Close()
			runtime.UnlockOSThread()
			stdout.Close()
			stderr.Close()
			return fmt.Errorf("netns: setns %s: %w", netNS, err)
		}
		ns.Close()
	}
	return execWithLogs(bwrap, args, stdout, stderr, envList(manifest.Env))
}

func execChroot(instDir, rootfs string, manifest types.CapsuleManifest) error {
	stdout, stderr, err := openLogs(instDir)
	if err != nil {
		return err
	}
	args := append([]string{}, manifest.Entrypoint[1:]...)
	args = append(args, manifest.Args...)
	if os.Geteuid() != 0 {
		return fmt.Errorf("chroot requires elevated privileges on this system. Try: sudo capper run %s", manifest.Name+".cap")
	}
	if err := syscall.Chroot(rootfs); err != nil {
		return err
	}
	if err := os.Chdir(manifest.WorkingDir); err != nil {
		return err
	}
	if err := syscall.Setgid(manifest.User.GID); err != nil {
		return err
	}
	if err := syscall.Setuid(manifest.User.UID); err != nil {
		return err
	}
	return execWithLogs(manifest.Entrypoint[0], args, stdout, stderr, envList(manifest.Env))
}

// Exec runs a command inside a running instance's rootfs.
// For crun/runc mode, instanceID is used for true namespace attach via
// the OCI runtime's exec interface. For bwrap/chroot, a new isolated
// process is spawned into the same rootfs (second-shell style).
func (r Runner) Exec(instanceID, rootfs, netNS string, command []string, user types.UserConfig) error {
	if len(command) == 0 {
		return fmt.Errorf("exec requires at least one command argument")
	}
	mode := r.mode()
	if mode == ModeCrun || mode == ModeRunc {
		bin, err := exec.LookPath(mode)
		if err == nil {
			return execOCIInteractive(bin, instanceID, command)
		}
		return fmt.Errorf("%s runtime requested but not found", mode)
	}
	if mode == ModeBwrap || mode == ModeAuto {
		bwrap, err := exec.LookPath("bwrap")
		if err == nil {
			return execBwrapInteractive(bwrap, rootfs, netNS, command, user)
		}
		if mode == ModeBwrap {
			return fmt.Errorf("bubblewrap runtime requested but bwrap was not found")
		}
	}
	return execChrootInteractive(rootfs, command, user)
}

func execOCIInteractive(runtime, containerID string, command []string) error {
	args := []string{"exec", containerID}
	args = append(args, command...)
	cmd := exec.Command(runtime, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func execBwrapInteractive(bwrap, rootfs, netNS string, command []string, user types.UserConfig) error {
	instDir := filepath.Dir(rootfs)
	args := []string{
		"--unshare-user",
		"--uid", strconv.Itoa(user.UID),
		"--gid", strconv.Itoa(user.GID),
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
	}
	if netNS == "" {
		args = append(args, "--unshare-net")
	}
	if hostname := rootfsHostname(rootfs); hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	args = append(args,
		"--bind", rootfs, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--chdir", "/",
	)
	args = appendProcOverrides(args, instDir)
	args = append(args, "--")
	args = append(args, command...)
	var cmd *exec.Cmd
	if netNS != "" {
		// nsenter --net=<path> works with any bind-mounted netns file — no
		// requirement for the file to live in /var/run/netns/.
		nsArgs := []string{"--net=" + network.NetNSPath(netNS), bwrap}
		nsArgs = append(nsArgs, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bwrap, args...)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func execChrootInteractive(rootfs string, command []string, user types.UserConfig) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("chroot exec requires elevated privileges on this system")
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
		Credential: &syscall.Credential{
			Uid: uint32(user.UID),
			Gid: uint32(user.GID),
		},
	}
	return cmd.Run()
}

// Connect opens an interactive shell inside a running instance.
// For crun/runc, this uses true namespace attach via the OCI runtime exec interface.
// For bwrap/chroot, a new isolated shell process is opened into the rootfs.
func (r Runner) Connect(instanceID, rootfs, netNS string, shells []string, user types.UserConfig) error {
	mode := r.mode()
	if mode == ModeCrun || mode == ModeRunc {
		bin, err := exec.LookPath(mode)
		if err == nil {
			return connectOCI(bin, instanceID, shells)
		}
		return fmt.Errorf("%s runtime requested but not found", mode)
	}
	for _, shell := range shells {
		if _, err := os.Stat(filepath.Join(rootfs, shell)); err == nil {
			if mode == ModeBwrap || mode == ModeAuto {
				bwrap, err := exec.LookPath("bwrap")
				if err == nil {
					return connectBwrap(bwrap, rootfs, netNS, shell, user)
				}
				if mode == ModeBwrap {
					return fmt.Errorf("bubblewrap runtime requested but bwrap was not found")
				}
			}
			if os.Geteuid() != 0 {
				return fmt.Errorf("chroot connect requires elevated privileges on this system")
			}
			cmd := exec.Command(shell)
			cmd.Dir = "/"
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Chroot: rootfs,
				Credential: &syscall.Credential{
					Uid: uint32(user.UID),
					Gid: uint32(user.GID),
				},
			}
			return cmd.Run()
		}
	}
	return fmt.Errorf("no usable shell found inside instance rootfs")
}

func connectOCI(runtime, containerID string, shells []string) error {
	for _, shell := range shells {
		cmd := exec.Command(runtime, "exec", "-t", containerID, shell)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no usable shell found in container %s", containerID)
}

func connectBwrap(bwrap, rootfs, netNS, shell string, user types.UserConfig) error {
	instDir := filepath.Dir(rootfs)
	args := []string{
		"--unshare-user",
		"--uid", strconv.Itoa(user.UID),
		"--gid", strconv.Itoa(user.GID),
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
	}
	if netNS == "" {
		args = append(args, "--unshare-net")
	}
	if hostname := rootfsHostname(rootfs); hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	args = append(args,
		"--bind", rootfs, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--chdir", "/",
	)
	args = appendProcOverrides(args, instDir)
	args = append(args, "--", shell)
	var cmd *exec.Cmd
	if netNS != "" {
		nsArgs := []string{"--net=" + network.NetNSPath(netNS), bwrap}
		nsArgs = append(nsArgs, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bwrap, args...)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// appendProcOverrides adds --ro-bind flags for any per-instance /proc masks.
func appendProcOverrides(args []string, instDir string) []string {
	overridesDir := filepath.Join(instDir, "proc-overrides")
	for _, name := range []string{"cpuinfo", "meminfo"} {
		src := filepath.Join(overridesDir, name)
		if _, err := os.Stat(src); err == nil {
			args = append(args, "--ro-bind", src, "/proc/"+name)
		}
	}
	return args
}

// WriteProcOverrides generates masked /proc files for the guest based on
// its resource allocation and writes them to instDir/proc-overrides/.
func WriteProcOverrides(instDir string, resources types.ResourceLimits) error {
	dir := filepath.Join(instDir, "proc-overrides")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	cpuinfo := "processor\t: 0\nvendor_id\t: GenuineIntel\ncpu family\t: 6\nmodel\t\t: 85\n" +
		"model name\t: Virtual CPU\nstepping\t: 0\ncpu MHz\t\t: 2000.000\ncache size\t: 4096 KB\n" +
		"physical id\t: 0\nsiblings\t: 1\ncore id\t\t: 0\ncpu cores\t: 1\n" +
		"flags\t\t: fpu sse sse2\nbogomips\t: 4000.00\n" +
		"clflush size\t: 64\ncache_alignment\t: 64\n" +
		"address sizes\t: 40 bits physical, 48 bits virtual\n\n"
	if err := os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte(cpuinfo), 0o644); err != nil {
		return err
	}
	memBytes := resources.MemoryBytes
	if memBytes <= 0 {
		memBytes = 512 * 1024 * 1024
	}
	memKB := memBytes / 1024
	freeKB := memKB / 2
	meminfo := fmt.Sprintf("MemTotal:       %8d kB\nMemFree:        %8d kB\nMemAvailable:   %8d kB\n"+
		"Buffers:               0 kB\nCached:                0 kB\nSwapTotal:             0 kB\nSwapFree:              0 kB\n",
		memKB, freeKB, freeKB)
	return os.WriteFile(filepath.Join(dir, "meminfo"), []byte(meminfo), 0o644)
}

func rootfsHostname(rootfs string) string {
	data, err := os.ReadFile(filepath.Join(rootfs, "etc", "hostname"))
	if err != nil {
		return ""
	}
	hostname := strings.TrimSpace(string(data))
	if hostname == "" || strings.ContainsAny(hostname, "/\x00\r\n\t ") {
		return ""
	}
	return hostname
}

// Stop terminates a running instance. For crun/runc mode, the OCI runtime's
// kill command is used for clean shutdown. For all modes, the tracked PID is
// used as a fallback.
func (r Runner) Stop(instanceID string, pid int, timeout time.Duration, killNow bool) error {
	mode := r.mode()
	if (mode == ModeCrun || mode == ModeRunc) && instanceID != "" {
		bin, err := exec.LookPath(mode)
		if err == nil {
			return stopOCI(bin, instanceID, pid, timeout, killNow)
		}
	}
	return stopByPID(pid, timeout, killNow)
}

func stopOCI(runtime, containerID string, pid int, timeout time.Duration, killNow bool) error {
	sig := "SIGTERM"
	if killNow {
		sig = "SIGKILL"
	}
	if err := runOCICommand(runtime, "kill", containerID, sig); err != nil {
		// If the OCI kill fails (container not in state, already stopped), fall back.
		return stopByPID(pid, timeout, killNow)
	}
	if killNow {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !Alive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if Alive(pid) {
		_ = runOCICommand(runtime, "kill", containerID, "SIGKILL")
	}
	return nil
}

func stopByPID(pid int, timeout time.Duration, killNow bool) error {
	if pid <= 0 || !Alive(pid) {
		return nil
	}
	sig := syscall.SIGTERM
	if killNow {
		sig = syscall.SIGKILL
	}
	if err := killProcessGroup(pid, sig); err != nil && err != syscall.ESRCH {
		return err
	}
	if killNow {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !Alive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if Alive(pid) {
		if err := killProcessGroup(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return err
		}
	}
	return nil
}

func runOCICommand(runtime string, args ...string) error {
	return exec.Command(runtime, args...).Run()
}

func (r Runner) mode() string {
	switch r.Mode {
	case ModeBwrap, ModeChroot, ModeCrun, ModeRunc:
		return r.Mode
	default:
		return ModeAuto
	}
}

func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}
	return !isZombie(pid)
}

func envList(env map[string]string) []string {
	// Start from a minimal clean baseline — never inherit host environment into
	// the capsule (avoids leaking LD_PRELOAD, proxy credentials, etc.).
	out := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func isZombie(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	fields := strings.Fields(string(data))
	return len(fields) > 2 && fields[2] == "Z"
}

func openLogs(instDir string) (*os.File, *os.File, error) {
	stdout, err := os.OpenFile(filepath.Join(instDir, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	stderr, err := os.OpenFile(filepath.Join(instDir, "stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		stdout.Close()
		return nil, nil, err
	}
	return stdout, stderr, nil
}

func writeLaunchSpec(path string, spec launchSpec) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func writeStartupError(instDir string, err error) {
	if instDir == "" || err == nil {
		return
	}
	_ = os.WriteFile(filepath.Join(instDir, "startup-error"), []byte(err.Error()+"\n"), 0o644)
}

func applyResourceLimits(limits types.ResourceLimits, enforcePids bool) error {
	if limits.MemoryBytes > 0 {
		if err := setLimit(unix.RLIMIT_AS, uint64(limits.MemoryBytes)); err != nil {
			return fmt.Errorf("set memory limit: %w", err)
		}
	}
	if limits.CPUTimeSecs > 0 {
		if err := setLimit(unix.RLIMIT_CPU, uint64(limits.CPUTimeSecs)); err != nil {
			return fmt.Errorf("set CPU time limit: %w", err)
		}
	}
	if enforcePids && limits.MaxProcesses > 0 {
		if err := setLimit(unix.RLIMIT_NPROC, uint64(limits.MaxProcesses)); err != nil {
			return fmt.Errorf("set process limit: %w", err)
		}
	}
	if limits.FileSizeBytes > 0 {
		if err := setLimit(unix.RLIMIT_FSIZE, uint64(limits.FileSizeBytes)); err != nil {
			return fmt.Errorf("set file size limit: %w", err)
		}
	}
	return nil
}

func setLimit(resource int, value uint64) error {
	return unix.Setrlimit(resource, &unix.Rlimit{Cur: value, Max: value})
}

func execWithLogs(path string, args []string, stdout, stderr *os.File, env []string) error {
	// Open /dev/null for stdin before any dup2 so we can report errors while
	// file handles are still valid.
	devNull, err := os.OpenFile("/dev/null", os.O_RDONLY, 0)
	if err != nil {
		stdout.Close()
		stderr.Close()
		return err
	}
	if err := syscall.Dup2(int(devNull.Fd()), 0); err != nil {
		devNull.Close()
		stdout.Close()
		stderr.Close()
		return err
	}
	if err := syscall.Dup2(int(stdout.Fd()), 1); err != nil {
		devNull.Close()
		stdout.Close()
		stderr.Close()
		return err
	}
	if err := syscall.Dup2(int(stderr.Fd()), 2); err != nil {
		devNull.Close()
		stdout.Close()
		stderr.Close()
		return err
	}
	// Close originals BEFORE syscall.Exec — defer would never run since Exec
	// replaces the process image, leaking the host-side fds into the capsule.
	devNull.Close()
	stdout.Close()
	stderr.Close()
	return syscall.Exec(path, append([]string{path}, args...), env)
}

func killProcessGroup(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pid, sig); err == nil || err != syscall.ESRCH {
		return err
	}
	return syscall.Kill(pid, sig)
}
