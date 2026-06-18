package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

// ApplyScript pipes the nft script to `nft -f -`.
// Requires CAP_NET_ADMIN or root.
func ApplyScript(script string) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nftables: apply failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteChain removes the per-network chain from the capper table.
// Errors are suppressed if the chain does not exist.
func DeleteChain(chainName string) error {
	// Flush rules first (chain can't be deleted while it has rules).
	flushCmd := exec.Command("nft", "flush", "chain", "inet", "capper", chainName)
	if out, err := flushCmd.CombinedOutput(); err != nil {
		// Ignore "no such table/chain" errors.
		if !isNoSuchError(string(out)) {
			return fmt.Errorf("nftables: flush chain %s: %w\n%s", chainName, err, strings.TrimSpace(string(out)))
		}
	}

	delCmd := exec.Command("nft", "delete", "chain", "inet", "capper", chainName)
	if out, err := delCmd.CombinedOutput(); err != nil {
		if !isNoSuchError(string(out)) {
			return fmt.Errorf("nftables: delete chain %s: %w\n%s", chainName, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func isNoSuchError(out string) bool {
	return strings.Contains(out, "No such file") ||
		strings.Contains(out, "no such table") ||
		strings.Contains(out, "no such chain") ||
		strings.Contains(out, "ENOENT")
}
