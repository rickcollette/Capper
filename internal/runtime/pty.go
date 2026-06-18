package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"capper/internal/network"
	"capper/internal/types"

	"github.com/creack/pty"
	"github.com/vishvananda/netns"
)

// defaultTerm is the terminal type advertised to interactive shells when the
// connecting client does not supply one. The web console (xterm.js) and modern
// CLI terminals are xterm-256color; without TERM set, curses apps such as top,
// less, and vi abort with "TERM environment variable not set."
const defaultTerm = "xterm-256color"

// shellTerm resolves the TERM value to use for an interactive session: an
// explicit override wins, otherwise the default.
func shellTerm(term string) string {
	if term != "" {
		return term
	}
	return defaultTerm
}

// StartShellPTY starts an interactive shell with a PTY attached.
// Caller must close the returned file when done.
func (r Runner) StartShellPTY(instanceID, rootfs, shell, netNS string, user types.UserConfig, term string) (*exec.Cmd, *os.File, error) {
	cmd, err := r.buildShellCmd(instanceID, rootfs, shell, netNS, user, shellTerm(term))
	if err != nil {
		return nil, nil, err
	}

	if netNS != "" {
		// Enter the target network namespace in the current OS thread before
		// fork+exec'ing bwrap, then restore the host netns afterward.
		// This avoids needing cap_sys_admin ambient (nsenter would require it);
		// the parent already has cap_sys_admin in its effective set from file caps.
		runtime.LockOSThread()
		origNS, err := netns.Get()
		if err != nil {
			runtime.UnlockOSThread()
			return nil, nil, fmt.Errorf("netns: get current: %w", err)
		}
		targetNS, err := netns.GetFromPath(network.NetNSPath(netNS))
		if err != nil {
			origNS.Close()
			runtime.UnlockOSThread()
			return nil, nil, fmt.Errorf("netns: open %s: %w", netNS, err)
		}
		setErr := netns.Set(targetNS)
		targetNS.Close()
		if setErr != nil {
			origNS.Close()
			runtime.UnlockOSThread()
			return nil, nil, fmt.Errorf("netns: enter %s: %w", netNS, setErr)
		}

		f, startErr := pty.Start(cmd)

		// Always restore the host netns before unlocking the thread.
		_ = netns.Set(origNS)
		origNS.Close()
		runtime.UnlockOSThread()

		if startErr != nil {
			return nil, nil, fmt.Errorf("pty start: %w", startErr)
		}
		return cmd, f, nil
	}

	f, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("pty start: %w", err)
	}
	return cmd, f, nil
}

func (r Runner) buildShellCmd(instanceID, rootfs, shell, netNS string, user types.UserConfig, term string) (*exec.Cmd, error) {
	mode := r.mode()
	if mode == ModeCrun || mode == ModeRunc {
		bin, err := exec.LookPath(mode)
		if err != nil {
			return nil, fmt.Errorf("%s runtime requested but not found", mode)
		}
		args := []string{"exec", "-t", "--env", "TERM=" + term, instanceID, shell, "-l"}
		return exec.Command(bin, args...), nil
	}
	if mode == ModeBwrap || mode == ModeAuto {
		bwrap, err := exec.LookPath("bwrap")
		if err == nil {
			return buildBwrapShellCmd(bwrap, rootfs, shell, netNS, user, term), nil
		}
		if mode == ModeBwrap {
			return nil, fmt.Errorf("bubblewrap runtime requested but bwrap was not found")
		}
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("chroot exec requires elevated privileges on this system")
	}
	cmd := exec.Command(shell, "-l")
	cmd.Dir = "/"
	// Login shell sources /etc/profile for PATH/PS1; add TERM so curses apps work.
	cmd.Env = []string{"TERM=" + term}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
		Credential: &syscall.Credential{
			Uid: uint32(user.UID),
			Gid: uint32(user.GID),
		},
	}
	return cmd, nil
}

func buildBwrapShellCmd(bwrap, rootfs, shell, netNS string, user types.UserConfig, term string) *exec.Cmd {
	instDir := filepath.Dir(rootfs)
	args := []string{
		"--setenv", "TERM", term,
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
	// Login shell so /etc/profile (PATH, PS1) is sourced for the interactive session.
	args = append(args, "--", shell, "-l")
	// netNS entry is handled by StartShellPTY (setns before fork); bwrap sees
	// the inherited netns and does not need --unshare-net or an nsenter wrapper.
	return exec.Command(bwrap, args...)
}

// PickShell returns the first usable shell inside rootfs.
func PickShell(rootfs string, preferred ...string) string {
	candidates := append(preferred, "/bin/bash", "/bin/sh", "/busybox/sh")
	for _, shell := range candidates {
		if shell == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(rootfs, shell)); err == nil {
			return shell
		}
	}
	return "/bin/sh"
}
