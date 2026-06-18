package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	capreg "capper/internal/registry"
	capstore "capper/internal/storage"
	"capper/internal/marketplace"
	capperdns "capper/internal/dns"
	"capper/internal/stack"
	"capper/internal/store"
	"capper/internal/types"
)

// ===========================================================================
// capper storage share
// ===========================================================================

func storageShareCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "share", Short: "manage file storage shares"}
	cmd.AddCommand(
		storageShareCreateCmd(opts),
		storageShareListCmd(opts),
		storageShareDeleteCmd(opts),
	)
	return cmd
}

func storageShareCreateCmd(opts *options) *cobra.Command {
	var path, mountPath, instance string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "create a file storage share",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				mgr := capstore.NewManager(st.Storage, storagePaths(st))
				sh, err := mgr.CreateShare(args[0], path, mountPath, instance)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(sh)
				}
				fmt.Printf("share/%s created (host: %s, mount: %s)\n", sh.Name, sh.HostPath, sh.MountPath)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "host filesystem path to share (required)")
	cmd.Flags().StringVar(&mountPath, "mount-path", "", "mount path inside the instance")
	cmd.Flags().StringVar(&instance, "instance", "", "target instance ID or name")
	return cmd
}

func storageShareListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list storage shares",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				mgr := capstore.NewManager(st.Storage, storagePaths(st))
				shares, err := mgr.ListShares()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(shares)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tHOST_PATH\tMOUNT_PATH\tINSTANCE\tCREATED")
				for _, sh := range shares {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						sh.Name, sh.HostPath, sh.MountPath, sh.InstanceID, shortTime(sh.CreatedAt))
				}
				return tw.Flush()
			})
		},
	}
}

func storageShareDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "delete a storage share",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				mgr := capstore.NewManager(st.Storage, storagePaths(st))
				if err := mgr.DeleteShare(args[0]); err != nil {
					return err
				}
				fmt.Printf("share/%s deleted\n", args[0])
				return nil
			})
		},
	}
}

// ===========================================================================
// capper storage bucket credentials
// ===========================================================================

func storageBucketCredentialsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "credentials NAME",
		Short: "generate access credentials for a bucket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				mgr := capstore.NewManager(st.Storage, storagePaths(st))
				b, err := mgr.GetBucket(args[0])
				if err != nil {
					return err
				}
				accessKey := capRandomHex(10)
				secretKey := capRandomHex(32)
				akName := "storage/" + b.Name + "/access-key"
				skName := "storage/" + b.Name + "/secret-key"
				if _, err := st.Secrets.Create(akName, opts.project, "bucket access key", accessKey); err != nil {
					return fmt.Errorf("store access key: %w", err)
				}
				if _, err := st.Secrets.Create(skName, opts.project, "bucket secret key", secretKey); err != nil {
					return fmt.Errorf("store secret key: %w", err)
				}
				return printJSON(map[string]string{
					"bucket":    b.Name,
					"accessKey": accessKey,
					"secretKey": secretKey,
				})
			})
		},
	}
}

// ===========================================================================
// capper registry token
// ===========================================================================

func registryTokenCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "token", Short: "manage registry auth tokens"}
	cmd.AddCommand(registryTokenCreateCmd(opts))
	return cmd
}

func registryTokenCreateCmd(opts *options) *cobra.Command {
	var registryName, ttlStr string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "issue a short-lived registry auth token",
		RunE: func(cmd *cobra.Command, args []string) error {
			ttl, err := time.ParseDuration(ttlStr)
			if err != nil {
				return fmt.Errorf("invalid --ttl: %w", err)
			}
			return withStore(opts, func(st *store.Store) error {
				tok, err := capreg.IssueToken(st.DB, registryName, ttl)
				if err != nil {
					return err
				}
				return printJSON(tok)
			})
		},
	}
	cmd.Flags().StringVar(&registryName, "registry", "", "registry name (empty = all registries)")
	cmd.Flags().StringVar(&ttlStr, "ttl", "24h", "token TTL (e.g. 1h, 24h)")
	return cmd
}

// ===========================================================================
// capper market
// ===========================================================================

func marketCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "manage the image marketplace",
	}
	cmd.AddCommand(
		marketListCmd(opts),
		marketSubmitCmd(opts),
		marketInspectCmd(opts),
		marketApproveCmd(opts),
		marketRejectCmd(opts),
		marketInstallCmd(opts),
		marketScanCmd(opts),
	)
	return cmd
}

func marketInstallCmd(opts *options) *cobra.Command {
	var params []string
	cmd := &cobra.Command{
		Use:   "install ID",
		Short: "install a marketplace listing as a stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:install", "project:"+opts.project); err != nil {
					return err
				}
				pm, perr := parseKeyValSpecs(params)
				if perr != nil {
					return fmt.Errorf("invalid --param: %w", perr)
				}
				err := ac.Store.Marketplace.Install(args[0], opts.project, pm, func(name, project, listingID string, p map[string]string) error {
					// Create a minimal stack record via the store's stack manager.
					tmpl := stack.StackTemplate{Name: name}
					_, serr := ac.Store.Stack.Apply(cmd.Context(), tmpl, project)
					return serr
				})
				if err != nil {
					return err
				}
				fmt.Printf("Listing %s installed into project %s\n", args[0], opts.project)
				return nil
			})
		},
	}
	cmd.Flags().StringArrayVar(&params, "param", nil, "key=value parameters passed to the stack template")
	return cmd
}

func marketScanCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "scan ID",
		Short: "run static scans on a marketplace listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:scan", "project:"+opts.project); err != nil {
					return err
				}
				results, err := ac.Store.Marketplace.RunStaticScans(args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(results)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TYPE\tSTATUS\tFINDINGS\tDETAIL")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Type, r.Status, r.Findings, r.Detail)
				}
				return tw.Flush()
			})
		},
	}
}

func marketListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list marketplace listings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:list", "project:"+opts.project); err != nil {
					return err
				}
				listings, err := ac.Store.Marketplace.List()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(listings)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tNAME\tVERSION\tSTATUS\tCREATED")
				for _, l := range listings {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						l.ID, l.Name, l.Version, l.Status, shortTime(l.CreatedAt))
				}
				return tw.Flush()
			})
		},
	}
}

func marketSubmitCmd(opts *options) *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "submit IMAGE",
		Short: "submit an image to the marketplace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:submit", "project:"+opts.project); err != nil {
					return err
				}
				l := marketplace.MarketplaceListing{
					Name:        args[0],
					Version:     "latest",
					Description: desc,
					Status:      marketplace.StatusPending,
				}
				if err := ac.Store.Marketplace.Insert(l); err != nil {
					return err
				}
				listings, _ := ac.Store.Marketplace.List()
				for _, existing := range listings {
					if existing.Name == l.Name && existing.Status == marketplace.StatusPending {
						l = existing
						break
					}
				}
				if opts.json {
					return printJSON(l)
				}
				fmt.Printf("Submitted %s (ID: %s)\n", l.Name, l.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&desc, "description", "", "listing description")
	return cmd
}

func marketInspectCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect ID",
		Short: "inspect a marketplace listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:list", "project:"+opts.project); err != nil {
					return err
				}
				l, err := ac.Store.Marketplace.Get(args[0])
				if err != nil {
					return err
				}
				return printJSON(l)
			})
		},
	}
}

func marketApproveCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "approve ID",
		Short: "approve a marketplace listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:approve", "project:"+opts.project); err != nil {
					return err
				}
				if err := ac.Store.Marketplace.UpdateStatus(args[0], marketplace.StatusApproved); err != nil {
					return err
				}
				fmt.Printf("Listing %s approved\n", args[0])
				return nil
			})
		},
	}
}

func marketRejectCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "reject ID",
		Short: "reject a marketplace listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("marketplace:moderate", "project:"+opts.project); err != nil {
					return err
				}
				if err := ac.Store.Marketplace.UpdateStatus(args[0], marketplace.StatusRejected); err != nil {
					return err
				}
				fmt.Printf("Listing %s rejected\n", args[0])
				return nil
			})
		},
	}
}

// ===========================================================================
// capper dns serve / healthcheck / trace
// ===========================================================================

func dnsServeCmd(opts *options) *cobra.Command {
	var netName, addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start DNS daemon bound to a network's gateway address",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if netName == "" {
					return fmt.Errorf("--network is required")
				}
				n, err := st.Networks.Get(netName, "")
				if err != nil {
					return fmt.Errorf("network %q not found: %w", netName, err)
				}
				listenAddr := addr
				if listenAddr == "" {
					listenAddr = n.Gateway + ":53"
				}
				resolver := capperdns.NewResolver(st.DNS, nil, []string{"8.8.8.8:53", "8.8.4.4:53"})
				daemon := capperdns.NewDaemon(listenAddr, resolver)
				if err := daemon.Start(); err != nil {
					return fmt.Errorf("dns daemon: %w", err)
				}
				fmt.Printf("DNS daemon listening on %s (network: %s, gateway: %s)\n",
					listenAddr, n.Name, n.Gateway)
				fmt.Println("Press Ctrl-C to stop.")
				ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
				defer cancel()
				<-ctx.Done()
				fmt.Println("\nShutting down DNS daemon...")
				return daemon.Stop()
			})
		},
	}
	cmd.Flags().StringVar(&netName, "network", "", "network name or ID (required)")
	cmd.Flags().StringVar(&addr, "addr", "", "listen address (default: GATEWAY:53)")
	return cmd
}

func dnsHealthcheckCmd(opts *options) *cobra.Command {
	var httpSpec string
	var interval int
	cmd := &cobra.Command{
		Use:   "healthcheck ZONE NAME",
		Short: "poll a DNS record's IP and mark unhealthy after 3 failures",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if httpSpec == "" {
				return fmt.Errorf("--http is required (e.g. :8080/health)")
			}
			return withStore(opts, func(st *store.Store) error {
				zone, name := args[0], args[1]
				mgr := capperdns.NewManager(st.DNS)
				z, err := mgr.GetZone(zone, "")
				if err != nil {
					return err
				}
				fqdn := strings.TrimSuffix(name+"."+z.Name, ".") + "."
				resolver := capperdns.NewResolver(st.DNS, nil, nil)
				failCount := 0
				ticker := time.NewTicker(time.Duration(interval) * time.Second)
				defer ticker.Stop()
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				defer signal.Stop(sigCh)
				fmt.Printf("Healthcheck polling %s every %ds via %s\n", fqdn, interval, httpSpec)
				for {
					select {
					case <-sigCh:
						return nil
					case <-ticker.C:
						rrs, _ := resolver.Query(fqdn, "A")
						if len(rrs) == 0 {
							fmt.Printf("  no records for %s, skipping\n", fqdn)
							continue
						}
						// Extract IP from the last field of the RR string.
						rrStr := rrs[0].String()
						parts := strings.Fields(rrStr)
						ipAddr := parts[len(parts)-1]
						url := "http://" + ipAddr + httpSpec
						resp, herr := http.Get(url) //nolint:noctx
						healthy := herr == nil && resp != nil && resp.StatusCode < 400
						if resp != nil {
							resp.Body.Close()
						}
						if !healthy {
							failCount++
							fmt.Printf("  FAIL (%d/3) %s\n", failCount, url)
							if failCount >= 3 {
								recs, lerr := st.DNS.LookupRecords(z.ID, fqdn, "A")
								if lerr == nil && len(recs) > 0 {
									_ = st.DNS.DeleteRecord(recs[0].ID)
									fmt.Printf("  UNHEALTHY: removed record %s\n", recs[0].ID)
								}
								failCount = 0
							}
						} else {
							failCount = 0
							fmt.Printf("  OK %s\n", url)
						}
					}
				}
			})
		},
	}
	cmd.Flags().StringVar(&httpSpec, "http", "", "HTTP path to poll, e.g. :8080/health")
	cmd.Flags().IntVar(&interval, "interval", 30, "poll interval in seconds")
	return cmd
}

func dnsTraceCmd(opts *options) *cobra.Command {
	var qtype string
	cmd := &cobra.Command{
		Use:   "trace FQDN",
		Short: "trace a DNS resolution with timing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				resolver := capperdns.NewResolver(st.DNS, nil, nil)
				start := time.Now()
				rrs, err := resolver.Query(args[0], qtype)
				elapsed := time.Since(start)
				if err != nil {
					return err
				}
				if opts.json {
					out := make([]string, 0, len(rrs))
					for _, rr := range rrs {
						out = append(out, rr.String())
					}
					return printJSON(map[string]any{
						"query":   args[0],
						"type":    qtype,
						"answers": out,
						"elapsed": elapsed.String(),
					})
				}
				fmt.Printf(";; Query: %s %s\n;; Elapsed: %s\n\n%s\n",
					args[0], qtype, elapsed, capperdns.FormatRRs(rrs))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&qtype, "type", "A", "record type: A, AAAA, CNAME, TXT")
	return cmd
}

// ===========================================================================
// port publish helpers (Block 15)
// ===========================================================================

// applyPortPublish adds iptables DNAT rules for each port mapping.
func applyPortPublish(instanceIP string, ports []types.PortMapping) {
	for _, p := range ports {
		if p.HostPort == 0 || p.ContainerPort == 0 {
			continue
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		dest := fmt.Sprintf("%s:%d", instanceIP, p.ContainerPort)
		checkArgs := []string{
			"-t", "nat", "-C", "PREROUTING",
			"-p", proto, "--dport", strconv.Itoa(p.HostPort),
			"-j", "DNAT", "--to-destination", dest,
		}
		if runIPTables(checkArgs...) == nil {
			continue
		}
		addArgs := []string{
			"-t", "nat", "-A", "PREROUTING",
			"-p", proto, "--dport", strconv.Itoa(p.HostPort),
			"-j", "DNAT", "--to-destination", dest,
		}
		_ = runIPTables(addArgs...)
	}
}

func runIPTables(args ...string) error {
	cmd := exec.Command("iptables", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v: %w\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// capRandomHex returns n random bytes as hex.
func capRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
