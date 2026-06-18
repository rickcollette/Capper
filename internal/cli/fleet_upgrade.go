package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// fleetCmd drives enterprise rolling upgrades of node agents against a remote
// control plane, using the cordon/drain/undrain API + per-node version
// convergence. The control plane is expected to already be upgraded first.
func fleetCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "manage rolling upgrades across a fleet of node agents",
	}
	cmd.AddCommand(fleetStatusCmd(opts), fleetUpgradeCmd(opts))
	return cmd
}

// fleetNode is the subset of the node record the upgrade driver needs.
type fleetNode struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Cordoned     bool   `json:"cordoned"`
	AgentVersion string `json:"agentVersion"`
	ZoneID       string `json:"zoneId"`
}

// fleetClient is a tiny control-plane REST client.
type fleetClient struct {
	base  string
	token string
	hc    *http.Client
}

func newFleetClient(base, token string) *fleetClient {
	return &fleetClient{base: strings.TrimRight(base, "/"), token: token, hc: &http.Client{Timeout: 30 * time.Second}}
}

func (c *fleetClient) do(method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *fleetClient) listNodes() ([]fleetNode, error) {
	data, err := c.do(http.MethodGet, "/api/v1/nodes", nil)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data []fleetNode `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func (c *fleetClient) cordon(node string) error {
	_, err := c.do(http.MethodPost, "/api/v1/nodes/"+node+"/cordon", nil)
	return err
}
func (c *fleetClient) drain(node string) error {
	_, err := c.do(http.MethodPost, "/api/v1/nodes/"+node+"/drain", nil)
	return err
}
func (c *fleetClient) undrain(node string) error {
	_, err := c.do(http.MethodPost, "/api/v1/nodes/"+node+"/undrain", nil)
	return err
}
func (c *fleetClient) health() error {
	_, err := c.do(http.MethodGet, "/api/v1/health", nil)
	return err
}

func controlURL(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("CAPPER_CONTROL_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func fleetToken(flag string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv("CAPPER_TOKEN")
}

func fleetStatusCmd(opts *options) *cobra.Command {
	var ctrlURL, token string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show each node's agent version and drain state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := newFleetClient(controlURL(ctrlURL), fleetToken(token))
			nodes, err := c.listNodes()
			if err != nil {
				return err
			}
			if opts.json {
				return printJSON(nodes)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "NODE\tVERSION\tSTATUS\tCORDONED")
			for _, n := range nodes {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", n.Name, orDash(n.AgentVersion), n.Status, n.Cordoned)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&ctrlURL, "control-url", "", "control-plane base URL (default $CAPPER_CONTROL_URL or http://localhost:8080)")
	cmd.Flags().StringVar(&token, "token", "", "bearer token (default $CAPPER_TOKEN)")
	return cmd
}

func fleetUpgradeCmd(opts *options) *cobra.Command {
	var ctrlURL, token, target, execTmpl string
	var batch int
	var timeout time.Duration
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "upgrade --target VERSION",
		Short: "rolling-upgrade node agents to a target version, batch by batch",
		Long: "Drains each node, runs the per-node upgrade command (--exec), waits for the\n" +
			"node to re-report the target version, then uncordons it — N nodes at a time,\n" +
			"with a control-plane health gate between batches. Stops on the first failure.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if target == "" {
				return fmt.Errorf("--target VERSION is required")
			}
			c := newFleetClient(controlURL(ctrlURL), fleetToken(token))
			nodes, err := c.listNodes()
			if err != nil {
				return err
			}
			var todo []fleetNode
			for _, n := range nodes {
				if n.AgentVersion != target {
					todo = append(todo, n)
				}
			}
			if len(todo) == 0 {
				fmt.Printf("All %d node(s) already at %s.\n", len(nodes), target)
				return nil
			}
			fmt.Printf("Upgrading %d/%d node(s) to %s (batch=%d):\n", len(todo), len(nodes), target, batch)
			for _, n := range todo {
				fmt.Printf("  %s (%s)\n", n.Name, orDash(n.AgentVersion))
			}
			if dryRun {
				fmt.Println("\n--dry-run: no changes made.")
				return nil
			}

			for i := 0; i < len(todo); i += batch {
				end := i + batch
				if end > len(todo) {
					end = len(todo)
				}
				for _, n := range todo[i:end] {
					if err := upgradeOneNode(c, n, target, execTmpl, timeout); err != nil {
						return fmt.Errorf("node %s: %w", n.Name, err)
					}
				}
				// Health-gate the control plane between batches.
				if err := c.health(); err != nil {
					return fmt.Errorf("control-plane health check failed after batch: %w", err)
				}
			}
			fmt.Printf("✓ fleet upgraded to %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&ctrlURL, "control-url", "", "control-plane base URL (default $CAPPER_CONTROL_URL or http://localhost:8080)")
	cmd.Flags().StringVar(&token, "token", "", "bearer token (default $CAPPER_TOKEN)")
	cmd.Flags().StringVar(&target, "target", "", "target agent version (required)")
	cmd.Flags().StringVar(&execTmpl, "exec", "", "per-node upgrade command; {node} is substituted (e.g. 'ssh {node} capper-self-upgrade')")
	cmd.Flags().IntVar(&batch, "batch", 1, "number of nodes to upgrade concurrently")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "per-node convergence timeout")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without making changes")
	return cmd
}

// upgradeOneNode drains, swaps (via --exec), waits for version convergence, and
// uncordons a single node.
func upgradeOneNode(c *fleetClient, n fleetNode, target, execTmpl string, timeout time.Duration) error {
	fmt.Printf("→ %s: cordon + drain\n", n.Name)
	if err := c.cordon(n.Name); err != nil {
		return fmt.Errorf("cordon: %w", err)
	}
	if err := c.drain(n.Name); err != nil {
		return fmt.Errorf("drain: %w", err)
	}

	if execTmpl != "" {
		cmdline := strings.ReplaceAll(execTmpl, "{node}", n.Name)
		fmt.Printf("→ %s: exec %q\n", n.Name, cmdline)
		ex := exec.Command("sh", "-c", cmdline)
		ex.Stdout, ex.Stderr = os.Stdout, os.Stderr
		if err := ex.Run(); err != nil {
			return fmt.Errorf("exec upgrade command: %w", err)
		}
	} else {
		fmt.Printf("→ %s: no --exec given; waiting for external upgrade to %s\n", n.Name, target)
	}

	// Wait for the node to re-report the target version via heartbeat.
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s to reach %s", n.Name, target)
		}
		nodes, err := c.listNodes()
		if err == nil {
			for _, cur := range nodes {
				if cur.Name == n.Name && cur.AgentVersion == target {
					goto converged
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
converged:
	fmt.Printf("→ %s: converged to %s, uncordon\n", n.Name, target)
	return c.undrain(n.Name)
}
