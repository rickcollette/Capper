package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"capper/internal/ipam"
	"capper/internal/store"
)

// ipPoolCmd implements `capper ip-pool`.
func ipPoolCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "ip-pool", Short: "manage routable IP pools"}
	cmd.AddCommand(ipPoolCreateCmd(opts), ipPoolListCmd(opts), ipPoolDeleteCmd(opts))
	return cmd
}

func ipPoolCreateCmd(opts *options) *cobra.Command {
	var cidr, scope, region, gateway, iface, usage string
	var noAuto bool
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "create a routable IP pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				mgr := ipam.NewManager(st.IPAM)
				pool, count, err := mgr.CreatePool(ipam.CreatePoolOptions{
					Pool: ipam.RoutableIPPool{
						Name: args[0], CIDR: cidr, Scope: scope, RegionID: region, Gateway: gateway,
						InterfaceName: iface, Usage: splitCSV(usage), Status: ipam.PoolActive,
						AllowAutoAllocate: !noAuto,
					},
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(map[string]any{"pool": pool, "addresses": count})
				}
				fmt.Printf("Pool created: %s (%s) — %d usable addresses\n", pool.Name, pool.ID, count)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&cidr, "cidr", "", "pool CIDR (required)")
	cmd.Flags().StringVar(&scope, "scope", "global", "pool scope")
	cmd.Flags().StringVar(&region, "region", "", "region (when scope=region)")
	cmd.Flags().StringVar(&gateway, "gateway", "", "gateway address to exclude")
	cmd.Flags().StringVar(&iface, "interface", "", "host interface name")
	cmd.Flags().StringVar(&usage, "usage", "", "comma-separated usage classes")
	cmd.Flags().BoolVar(&noAuto, "no-auto-allocate", false, "reserved-only pool (no auto allocation)")
	_ = cmd.MarkFlagRequired("cidr")
	return cmd
}

func ipPoolListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list IP pools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				pools, err := st.IPAM.ListPools()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(pools)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tCIDR\tSCOPE\tSTATUS\tUSAGE")
				for _, p := range pools {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.CIDR, p.Scope, p.Status, strings.Join(p.Usage, ","))
				}
				return tw.Flush()
			})
		},
	}
}

func ipPoolDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "delete an IP pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				pool, err := st.IPAM.GetPool(args[0])
				if err != nil {
					return err
				}
				if err := st.IPAM.DeletePool(pool.ID); err != nil {
					return err
				}
				fmt.Printf("Pool %q deleted.\n", args[0])
				return nil
			})
		},
	}
}

// ipCmd implements `capper ip`.
func ipCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "ip", Short: "manage routable IP addresses"}
	cmd.AddCommand(ipReserveCmd(opts), ipListCmd(opts), ipReleaseCmd(opts))
	return cmd
}

func ipReserveCmd(opts *options) *cobra.Command {
	var pool, purpose, address string
	var reserved bool
	cmd := &cobra.Command{
		Use:   "reserve NAME",
		Short: "reserve an IP from a pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				p, err := st.IPAM.GetPool(pool)
				if err != nil {
					return err
				}
				ip, err := ipam.NewManager(st.IPAM).Reserve(ipam.ReserveOptions{
					PoolID: p.ID, Project: opts.project, Name: args[0], Purpose: purpose,
					Address: address, Reserved: reserved,
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(ip)
				}
				fmt.Printf("Reserved %s as %q (%s)\n", ip.Address, ip.Name, ip.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&pool, "pool", "", "pool name or ID (required)")
	cmd.Flags().StringVar(&purpose, "purpose", "", "purpose (load-balancer, egress, passthrough, …)")
	cmd.Flags().StringVar(&address, "address", "", "specific address to reserve")
	cmd.Flags().BoolVar(&reserved, "reserved", false, "mark as a reserved (Elastic) IP")
	_ = cmd.MarkFlagRequired("pool")
	return cmd
}

func ipListCmd(opts *options) *cobra.Command {
	var pool, status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list IP addresses",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				poolID := ""
				if pool != "" {
					if p, err := st.IPAM.GetPool(pool); err == nil {
						poolID = p.ID
					}
				}
				ips, err := st.IPAM.ListIPs(poolID, status)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(ips)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ADDRESS\tSTATUS\tNAME\tPURPOSE\tTARGET")
				for _, ip := range ips {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", ip.Address, ip.Status, ip.Name, ip.Purpose, ip.TargetID)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&pool, "pool", "", "filter by pool")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	return cmd
}

func ipReleaseCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "release NAME",
		Short: "release a reserved IP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				ip, err := st.IPAM.GetIPByName(opts.project, args[0])
				if err != nil {
					return err
				}
				if err := ipam.NewManager(st.IPAM).Release(ip.ID); err != nil {
					return err
				}
				fmt.Printf("Released %s (%q)\n", ip.Address, args[0])
				return nil
			})
		},
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
