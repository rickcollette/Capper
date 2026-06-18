package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"capper/internal/store"
	"capper/internal/topology"
)

// ---------------------------------------------------------------------------
// capper realm
// ---------------------------------------------------------------------------

func realmCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "realm", Short: "manage realms"}
	cmd.AddCommand(
		realmListCmd(opts),
		realmCreateCmd(opts),
		realmGetCmd(opts),
		realmDeleteCmd(opts),
	)
	return cmd
}

func realmListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "list realms",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				realms, err := st.Topology.Store().ListRealms()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(realms)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tNAME\tSTATUS")
				for _, r := range realms {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Slug, r.Name, r.Status)
				}
				return tw.Flush()
			})
		},
	}
}

func realmCreateCmd(opts *options) *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "create a realm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r := topology.Realm{Slug: args[0], Name: name, Description: description}
				if r.Name == "" {
					r.Name = args[0]
				}
				if err := st.Topology.Store().InsertRealm(r); err != nil {
					return err
				}
				got, _ := st.Topology.Store().GetRealm(args[0])
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Created realm %s (%s)\n", got.Slug, got.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&description, "description", "", "description")
	return cmd
}

func realmGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get realm details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r, err := st.Topology.Store().GetRealm(args[0])
				if err != nil {
					return err
				}
				return printJSON(r)
			})
		},
	}
}

func realmDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a realm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeleteRealm(args[0])
			})
		},
	}
}

// ---------------------------------------------------------------------------
// capper region
// ---------------------------------------------------------------------------

func regionCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "region", Short: "manage regions"}
	cmd.AddCommand(
		regionListCmd(opts),
		regionCreateCmd(opts),
		regionGetCmd(opts),
		regionDeleteCmd(opts),
		regionDrainCmd(opts),
		regionEvacuateCmd(opts),
	)
	return cmd
}

func regionListCmd(opts *options) *cobra.Command {
	var realm string
	cmd := &cobra.Command{
		Use: "list", Short: "list regions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var realmID string
				if realm != "" {
					r, err := st.Topology.Store().GetRealm(realm)
					if err != nil {
						return err
					}
					realmID = r.ID
				}
				regions, err := st.Topology.Store().ListRegions(realmID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(regions)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tNAME\tLOCATION\tSTATUS")
				for _, rg := range regions {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", rg.ID, rg.Slug, rg.Name, rg.Location, rg.Status)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&realm, "realm", "", "filter by realm slug or ID")
	return cmd
}

func regionCreateCmd(opts *options) *cobra.Command {
	var realm, name, location, country, regionCode string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "create a region",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var realmID string
				if realm != "" {
					r, err := st.Topology.Store().GetRealm(realm)
					if err != nil {
						return fmt.Errorf("realm %q not found: %w", realm, err)
					}
					realmID = r.ID
				} else {
					r, err := st.Topology.DefaultRealm()
					if err == nil {
						realmID = r.ID
					}
				}
				rg := topology.Region{
					RealmID: realmID, Slug: args[0],
					Name: name, Location: location, Country: country, RegionCode: regionCode,
				}
				if rg.Name == "" {
					rg.Name = args[0]
				}
				if err := st.Topology.Store().InsertRegion(rg); err != nil {
					return err
				}
				got, _ := st.Topology.Store().GetRegion(args[0])
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Created region %s (%s)\n", got.Slug, got.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&realm, "realm", "", "realm slug or ID")
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&location, "location", "", "location description")
	cmd.Flags().StringVar(&country, "country", "", "country code")
	cmd.Flags().StringVar(&regionCode, "region-code", "", "region code")
	return cmd
}

func regionGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get region details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r, err := st.Topology.Store().GetRegion(args[0])
				if err != nil {
					return err
				}
				return printJSON(r)
			})
		},
	}
}

func regionDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a region",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeleteRegion(args[0])
			})
		},
	}
}

func regionDrainCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "drain <slug-or-id>", Short: "drain a region",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r, err := st.Topology.Store().GetRegion(args[0])
				if err != nil {
					return err
				}
				r.Status = topology.StatusDraining
				return st.Topology.Store().UpdateRegion(r)
			})
		},
	}
}

func regionEvacuateCmd(opts *options) *cobra.Command {
	var targetRegion string
	cmd := &cobra.Command{
		Use: "evacuate <slug-or-id>", Short: "evacuate a region to a target region",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				src, err := st.Topology.Store().GetRegion(args[0])
				if err != nil {
					return err
				}
				var targetID string
				if targetRegion != "" {
					tgt, err := st.Topology.Store().GetRegion(targetRegion)
					if err != nil {
						return fmt.Errorf("target region %q not found", targetRegion)
					}
					targetID = tgt.ID
				}
				plan := topology.MigrationPlan{
					RealmID:        src.RealmID,
					Name:           fmt.Sprintf("evacuate-%s", src.Slug),
					MigrationType:  "region-evacuate",
					SourceRegionID: src.ID,
					TargetRegionID: targetID,
				}
				if err := st.Topology.Store().InsertMigrationPlan(plan); err != nil {
					return err
				}
				src.Status = "evacuating"
				if err := st.Topology.Store().UpdateRegion(src); err != nil {
					return err
				}
				fmt.Printf("Region %s marked evacuating. Migration plan created.\n", src.Slug)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&targetRegion, "target-region", "", "target region slug or ID")
	return cmd
}

// ---------------------------------------------------------------------------
// capper zone
// ---------------------------------------------------------------------------

func zoneCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "zone", Short: "manage zones"}
	cmd.AddCommand(
		zoneListCmd(opts),
		zoneCreateCmd(opts),
		zoneGetCmd(opts),
		zoneDeleteCmd(opts),
		zoneCordonCmd(opts),
		zoneDrainCmd(opts),
	)
	return cmd
}

func zoneListCmd(opts *options) *cobra.Command {
	var region string
	cmd := &cobra.Command{
		Use: "list", Short: "list zones",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var regionID string
				if region != "" {
					r, err := st.Topology.Store().GetRegion(region)
					if err != nil {
						return err
					}
					regionID = r.ID
				}
				zones, err := st.Topology.Store().ListZones(regionID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(zones)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tNAME\tFAILURE-DOMAIN\tSTATUS")
				for _, z := range zones {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", z.ID, z.Slug, z.Name, z.FailureDomain, z.Status)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "filter by region slug or ID")
	return cmd
}

func zoneCreateCmd(opts *options) *cobra.Command {
	var region, realm, name, failureDomain, networkCIDR, controlURL string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "create a zone",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var regionID, realmID string
				if region != "" {
					r, err := st.Topology.Store().GetRegion(region)
					if err != nil {
						return fmt.Errorf("region %q: %w", region, err)
					}
					regionID = r.ID
					realmID = r.RealmID
				}
				if realm != "" {
					r, err := st.Topology.Store().GetRealm(realm)
					if err != nil {
						return fmt.Errorf("realm %q: %w", realm, err)
					}
					realmID = r.ID
				}
				z := topology.Zone{
					RealmID: realmID, RegionID: regionID, Slug: args[0],
					Name: name, FailureDomain: failureDomain,
					NetworkCIDR: networkCIDR, ControlURL: controlURL,
				}
				if z.Name == "" {
					z.Name = args[0]
				}
				if err := st.Topology.Store().InsertZone(z); err != nil {
					return err
				}
				got, _ := st.Topology.Store().GetZone(args[0])
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Created zone %s (%s)\n", got.Slug, got.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "region slug or ID")
	cmd.Flags().StringVar(&realm, "realm", "", "realm slug or ID")
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&failureDomain, "failure-domain", "", "failure domain (rack, power group, etc.)")
	cmd.Flags().StringVar(&networkCIDR, "network-cidr", "", "zone network CIDR")
	cmd.Flags().StringVar(&controlURL, "control-url", "", "zone controller URL")
	return cmd
}

func zoneGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get zone details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				z, err := st.Topology.Store().GetZone(args[0])
				if err != nil {
					return err
				}
				return printJSON(z)
			})
		},
	}
}

func zoneDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a zone",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeleteZone(args[0])
			})
		},
	}
}

func zoneCordonCmd(opts *options) *cobra.Command {
	var uncordon bool
	cmd := &cobra.Command{
		Use: "cordon <slug-or-id>", Short: "cordon (or uncordon) a zone",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				status := topology.StatusCordoned
				if uncordon {
					status = topology.StatusActive
				}
				return st.Topology.Store().UpdateZoneStatus(args[0], status)
			})
		},
	}
	cmd.Flags().BoolVar(&uncordon, "uncordon", false, "uncordon instead of cordon")
	return cmd
}

func zoneDrainCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "drain <slug-or-id>", Short: "drain a zone",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().UpdateZoneStatus(args[0], topology.StatusDraining)
			})
		},
	}
}

// ---------------------------------------------------------------------------
// capper node
// ---------------------------------------------------------------------------

func nodeCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "node", Short: "manage topology nodes"}
	cmd.AddCommand(
		nodeListCmd(opts),
		nodeRegisterCmd(opts),
		nodeGetCmd(opts),
		nodeDeleteCmd(opts),
		nodeCordonCmd(opts),
		nodeDrainCmd(opts),
		nodeJoinCmd(opts),
		nodeApproveCmd(opts),
	)
	return cmd
}

func nodeJoinCmd(opts *options) *cobra.Command {
	var token, address, agentVersion string
	var cpuCount int
	var memoryBytes, diskBytes int64
	var gpuCount int
	var roles, labelPairs []string
	cmd := &cobra.Command{
		Use:   "join <name>",
		Short: "register this node via a join token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				jt, err := st.Topology.Store().ConsumeJoinToken(token)
				if err != nil {
					return fmt.Errorf("invalid or expired join token: %w", err)
				}
				labels := parseLabels(labelPairs)
				node := topology.Node{
					RealmID:      jt.RealmID,
					RegionID:     jt.RegionID,
					ZoneID:       jt.ZoneID,
					Name:         args[0],
					Slug:         args[0],
					Address:      address,
					Status:       "pending",
					CPUCount:     cpuCount,
					MemoryBytes:  memoryBytes,
					DiskBytes:    diskBytes,
					GPUCount:     gpuCount,
					AgentVersion: agentVersion,
					Labels:       labels,
				}
				created, err := st.Topology.Store().InsertNode(node)
				if err != nil {
					return err
				}
				nodeRoles := jt.Roles
				if len(roles) > 0 {
					nodeRoles = roles
				}
				if len(nodeRoles) > 0 {
					_ = st.Topology.Store().SetNodeRoles(created.ID, nodeRoles)
				}
				bearer, tok, err := st.IAM.Issue(created.Name, "node", created.ID, 365*24*time.Hour)
				if err != nil {
					return fmt.Errorf("node registered but token issuance failed: %w", err)
				}
				certPEM, _, certErr := st.Certs.IssueNodeCert(created.Name+".node.capper.internal",
					[]string{created.Name + ".node.capper.internal", created.Address}, 365*24*time.Hour)
				if opts.json {
					return printJSON(map[string]any{
						"node":    created,
						"token":   tok,
						"bearer":  bearer,
						"certPEM": string(certPEM),
					})
				}
				fmt.Printf("Node registered: %s (id=%s status=%s)\n", created.Slug, created.ID, created.Status)
				fmt.Printf("IAM token: %s\n", bearer)
				if certErr == nil && len(certPEM) > 0 {
					fmt.Printf("Identity cert issued (common name: %s.node.capper.internal)\n", created.Name)
				}
				fmt.Println("Node is pending approval. Run: capper node approve", created.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "join token (required)")
	cmd.Flags().StringVar(&address, "address", "", "node address/IP")
	cmd.Flags().StringVar(&agentVersion, "agent-version", "", "agent version string")
	cmd.Flags().IntVar(&cpuCount, "cpu", 0, "CPU count")
	cmd.Flags().Int64Var(&memoryBytes, "memory", 0, "memory in bytes")
	cmd.Flags().Int64Var(&diskBytes, "disk", 0, "disk in bytes")
	cmd.Flags().IntVar(&gpuCount, "gpu", 0, "GPU count")
	cmd.Flags().StringArrayVar(&roles, "role", nil, "node roles (repeatable)")
	cmd.Flags().StringArrayVar(&labelPairs, "label", nil, "key=value labels (repeatable)")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func nodeApproveCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "approve <node-id-or-slug>",
		Short: "approve a pending node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if err := st.Topology.Store().UpdateNodeStatus(args[0], topology.StatusReady); err != nil {
					return err
				}
				node, err := st.Topology.Store().GetNode(args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(node)
				}
				fmt.Printf("Node approved: %s (status=%s)\n", node.Slug, node.Status)
				return nil
			})
		},
	}
}

func nodeListCmd(opts *options) *cobra.Command {
	var zone, region string
	cmd := &cobra.Command{
		Use: "list", Short: "list nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var zoneID string
				if zone != "" {
					z, err := st.Topology.Store().GetZone(zone)
					if err != nil {
						return err
					}
					zoneID = z.ID
				} else if region != "" {
					reg, err := st.Topology.Store().GetRegion(region)
					if err != nil {
						return err
					}
					zones, _ := st.Topology.Store().ListZones(reg.ID)
					var all []topology.Node
					for _, z := range zones {
						ns, _ := st.Topology.Store().ListNodes(z.ID)
						all = append(all, ns...)
					}
					if opts.json {
						return printJSON(all)
					}
					printNodeTable(all)
					return nil
				}
				nodes, err := st.Topology.Store().ListNodes(zoneID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(nodes)
				}
				printNodeTable(nodes)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&zone, "zone", "", "filter by zone slug or ID")
	cmd.Flags().StringVar(&region, "region", "", "filter by region slug or ID")
	return cmd
}

func printNodeTable(nodes []topology.Node) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSLUG\tADDRESS\tCPU\tMEMORY\tSTATUS")
	for _, n := range nodes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			n.ID, n.Slug, n.Address, n.CPUCount, humanSize(n.MemoryBytes), n.Status)
	}
	_ = tw.Flush()
}

func nodeRegisterCmd(opts *options) *cobra.Command {
	var zone, region, realm, address string
	var cpuCount int
	var memoryBytes, diskBytes int64
	var labels []string
	cmd := &cobra.Command{
		Use: "register <slug>", Short: "register a node",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var zoneID, regionID, realmID string
				if zone != "" {
					z, err := st.Topology.Store().GetZone(zone)
					if err != nil {
						return fmt.Errorf("zone %q: %w", zone, err)
					}
					zoneID, regionID, realmID = z.ID, z.RegionID, z.RealmID
				} else if region != "" {
					r, err := st.Topology.Store().GetRegion(region)
					if err != nil {
						return fmt.Errorf("region %q: %w", region, err)
					}
					regionID, realmID = r.ID, r.RealmID
				}
				if realm != "" {
					r, err := st.Topology.Store().GetRealm(realm)
					if err != nil {
						return fmt.Errorf("realm %q: %w", realm, err)
					}
					realmID = r.ID
				}
				lblMap := parseLabels(labels)
				n := topology.Node{
					RealmID: realmID, RegionID: regionID, ZoneID: zoneID,
					Slug: args[0], Name: args[0], Address: address,
					CPUCount: cpuCount, MemoryBytes: memoryBytes, DiskBytes: diskBytes,
					Labels: lblMap,
				}
				got, err := st.Topology.Store().InsertNode(n)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Registered node %s (%s)\n", got.Slug, got.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&zone, "zone", "", "zone slug or ID")
	cmd.Flags().StringVar(&region, "region", "", "region slug or ID")
	cmd.Flags().StringVar(&realm, "realm", "", "realm slug or ID")
	cmd.Flags().StringVar(&address, "address", "127.0.0.1", "node address")
	cmd.Flags().IntVar(&cpuCount, "cpu", 0, "CPU count")
	cmd.Flags().Int64Var(&memoryBytes, "memory", 0, "memory in bytes")
	cmd.Flags().Int64Var(&diskBytes, "disk", 0, "disk in bytes")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "labels key=value")
	return cmd
}

func nodeGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get node details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				n, err := st.Topology.Store().GetNode(args[0])
				if err != nil {
					return err
				}
				return printJSON(n)
			})
		},
	}
}

func nodeDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a node",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeleteNode(args[0])
			})
		},
	}
}

func nodeCordonCmd(opts *options) *cobra.Command {
	var uncordon bool
	cmd := &cobra.Command{
		Use: "cordon <slug-or-id>", Short: "cordon (or uncordon) a node",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				status := topology.StatusCordoned
				if uncordon {
					status = topology.StatusReady
				}
				return st.Topology.Store().UpdateNodeStatus(args[0], status)
			})
		},
	}
	cmd.Flags().BoolVar(&uncordon, "uncordon", false, "uncordon instead of cordon")
	return cmd
}

func nodeDrainCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "drain <slug-or-id>", Short: "drain a node",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().UpdateNodeStatus(args[0], topology.StatusDraining)
			})
		},
	}
}

// ---------------------------------------------------------------------------
// capper vpc
// ---------------------------------------------------------------------------

func vpcCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "vpc", Short: "manage VPCs"}
	cmd.AddCommand(
		vpcListCmd(opts),
		vpcCreateCmd(opts),
		vpcGetCmd(opts),
		vpcDeleteCmd(opts),
		vpcSubnetCmd(opts),
	)
	return cmd
}

func vpcListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "list VPCs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				vpcs, err := st.Topology.Store().ListVPCs(opts.project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(vpcs)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tNAME\tCIDR\tMOBILITY\tSTATUS")
				for _, v := range vpcs {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						v.ID, v.Slug, v.Name, v.CIDR, v.MobilityPolicy, v.Status)
				}
				return tw.Flush()
			})
		},
	}
}

func vpcCreateCmd(opts *options) *cobra.Command {
	var name, cidr, homeRegion, mobility string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "create a VPC",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var homeRegionID, realmID string
				if homeRegion != "" {
					r, err := st.Topology.Store().GetRegion(homeRegion)
					if err != nil {
						return fmt.Errorf("region %q: %w", homeRegion, err)
					}
					homeRegionID, realmID = r.ID, r.RealmID
				} else {
					if realm, err := st.Topology.DefaultRealm(); err == nil {
						realmID = realm.ID
					}
				}
				v := topology.VPC{
					RealmID: realmID, Project: opts.project, Slug: args[0],
					Name: name, CIDR: cidr, HomeRegionID: homeRegionID,
					MobilityPolicy: mobility,
				}
				if v.Name == "" {
					v.Name = args[0]
				}
				if err := st.Topology.Store().InsertVPC(v); err != nil {
					return err
				}
				got, _ := st.Topology.Store().GetVPC(opts.project, args[0])
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Created VPC %s (%s) cidr=%s\n", got.Slug, got.ID, got.CIDR)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&cidr, "cidr", "", "VPC CIDR block")
	cmd.Flags().StringVar(&homeRegion, "home-region", "", "home region slug or ID")
	cmd.Flags().StringVar(&mobility, "mobility", topology.MobilityManual, "mobility policy")
	return cmd
}

func vpcGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get VPC details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				v, err := st.Topology.Store().GetVPC(opts.project, args[0])
				if err != nil {
					return err
				}
				return printJSON(v)
			})
		},
	}
}

func vpcDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a VPC",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeleteVPC(opts.project, args[0])
			})
		},
	}
}

func vpcSubnetCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "subnet", Short: "manage VPC subnets"}
	cmd.AddCommand(
		vpcSubnetListCmd(opts),
		vpcSubnetCreateCmd(opts),
	)
	return cmd
}

func vpcSubnetListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list <vpc>", Short: "list subnets in a VPC",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				vpc, err := st.Topology.Store().GetVPC(opts.project, args[0])
				if err != nil {
					return err
				}
				subnets, err := st.Topology.Store().ListSubnets(vpc.ID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(subnets)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tCIDR\tGATEWAY\tMODE\tSTATUS")
				for _, s := range subnets {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						s.ID, s.Slug, s.CIDR, s.Gateway, s.Mode, s.Status)
				}
				return tw.Flush()
			})
		},
	}
}

func vpcSubnetCreateCmd(opts *options) *cobra.Command {
	var zone, name, cidr, gateway, mode string
	cmd := &cobra.Command{
		Use: "create <vpc> <slug>", Short: "create a subnet",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				vpc, err := st.Topology.Store().GetVPC(opts.project, args[0])
				if err != nil {
					return fmt.Errorf("vpc %q: %w", args[0], err)
				}
				var zoneID, regionID string
				if zone != "" {
					z, err := st.Topology.Store().GetZone(zone)
					if err != nil {
						return fmt.Errorf("zone %q: %w", zone, err)
					}
					zoneID, regionID = z.ID, z.RegionID
				}
				sub := topology.VPCSubnet{
					VPCID: vpc.ID, RealmID: vpc.RealmID, RegionID: regionID, ZoneID: zoneID,
					Slug: args[1], Name: name, CIDR: cidr, Gateway: gateway, Mode: mode,
				}
				if sub.Name == "" {
					sub.Name = args[1]
				}
				if err := st.Topology.Store().InsertSubnet(sub); err != nil {
					return err
				}
				fmt.Printf("Created subnet %s in vpc %s\n", sub.Slug, vpc.Slug)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&zone, "zone", "", "zone slug or ID")
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&cidr, "cidr", "", "subnet CIDR")
	cmd.Flags().StringVar(&gateway, "gateway", "", "gateway IP")
	cmd.Flags().StringVar(&mode, "mode", "nat", "subnet mode (nat, isolated, routed, overlay, host)")
	return cmd
}

// ---------------------------------------------------------------------------
// capper placement
// ---------------------------------------------------------------------------

func placementCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "placement", Short: "manage placement policies"}
	cmd.AddCommand(
		placementListCmd(opts),
		placementCreateCmd(opts),
		placementGetCmd(opts),
		placementDeleteCmd(opts),
	)
	return cmd
}

func placementListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "list placement policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				policies, err := st.Topology.Store().ListPlacementPolicies(opts.project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(policies)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSLUG\tNAME\tSTRATEGY\tSCOPE")
				for _, p := range policies {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.Slug, p.Name, p.Strategy, p.Scope)
				}
				return tw.Flush()
			})
		},
	}
}

func placementCreateCmd(opts *options) *cobra.Command {
	var name, strategy, scope string
	var minZones, minRegions int
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "create a placement policy",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var realmID string
				if r, err := st.Topology.DefaultRealm(); err == nil {
					realmID = r.ID
				}
				p := topology.PlacementPolicy{
					RealmID: realmID, Project: opts.project, Slug: args[0],
					Name: name, Strategy: strategy, Scope: scope,
					MinZones: minZones, MinRegions: minRegions,
				}
				if p.Name == "" {
					p.Name = args[0]
				}
				if err := st.Topology.Store().InsertPlacementPolicy(p); err != nil {
					return err
				}
				got, _ := st.Topology.Store().GetPlacementPolicy(opts.project, args[0])
				if opts.json {
					return printJSON(got)
				}
				fmt.Printf("Created placement policy %s (%s)\n", got.Slug, got.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&strategy, "strategy", topology.StrategySpreadZones, "placement strategy")
	cmd.Flags().StringVar(&scope, "scope", "region", "scope (region, zone, realm)")
	cmd.Flags().IntVar(&minZones, "min-zones", 1, "minimum zones")
	cmd.Flags().IntVar(&minRegions, "min-regions", 1, "minimum regions")
	return cmd
}

func placementGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <slug-or-id>", Short: "get placement policy details",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				p, err := st.Topology.Store().GetPlacementPolicy(opts.project, args[0])
				if err != nil {
					return err
				}
				return printJSON(p)
			})
		},
	}
}

func placementDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <slug-or-id>", Short: "delete a placement policy",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.Topology.Store().DeletePlacementPolicy(opts.project, args[0])
			})
		},
	}
}

// ---------------------------------------------------------------------------
// capper scheduler
// ---------------------------------------------------------------------------

func schedulerCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "scheduler", Short: "simulate and inspect the region scheduler"}
	cmd.AddCommand(
		schedulerSimulateCmd(opts),
		schedulerCapacityCmd(opts),
	)
	return cmd
}

func schedulerSimulateCmd(opts *options) *cobra.Command {
	var region, zone, strategy, image, instanceType string
	var minZones, cpuReq int
	var memReq int64
	var gpuReq bool
	var labels []string
	cmd := &cobra.Command{
		Use: "simulate", Short: "simulate a placement decision",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				req := topology.PlacementRequest{
					Project:      opts.project,
					Image:        image,
					InstanceType: instanceType,
					CPURequired:  cpuReq,
					MemoryBytes:  memReq,
					GPURequired:  gpuReq,
					Region:       region,
					Zone:         zone,
					Strategy:     strategy,
					MinZones:     minZones,
					RequireLabel: parseLabels(labels),
				}
				sched := topology.NewScheduler(st.Topology.Store())
				result := sched.Simulate(cmd.Context(), req)
				if opts.json {
					return printJSON(result)
				}
				if !result.Allowed {
					fmt.Println("No eligible nodes found.")
				} else {
					fmt.Printf("Candidates (%d):\n", len(result.Candidates))
					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "  REGION\tZONE\tNODE\tSCORE\tREASONS")
					for _, c := range result.Candidates {
						fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\t%s\n",
							c.Region, c.Zone, c.Node, c.Score, strings.Join(c.Reasons, ","))
					}
					_ = tw.Flush()
				}
				if len(result.Rejections) > 0 {
					fmt.Printf("\nRejected (%d):\n", len(result.Rejections))
					for _, rj := range result.Rejections {
						fmt.Printf("  %s: %s\n", rj.Node, rj.Reason)
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "preferred region")
	cmd.Flags().StringVar(&zone, "zone", "", "preferred zone")
	cmd.Flags().StringVar(&strategy, "strategy", topology.StrategySpreadZones, "placement strategy")
	cmd.Flags().StringVar(&image, "image", "", "image name")
	cmd.Flags().StringVar(&instanceType, "instance-type", "", "instance type")
	cmd.Flags().IntVar(&cpuReq, "cpu", 0, "minimum CPU count required")
	cmd.Flags().Int64Var(&memReq, "memory", 0, "minimum memory bytes required")
	cmd.Flags().BoolVar(&gpuReq, "gpu", false, "require GPU")
	cmd.Flags().IntVar(&minZones, "min-zones", 0, "minimum zones to spread across")
	cmd.Flags().StringSliceVar(&labels, "require-label", nil, "required node labels key=value")
	return cmd
}

func schedulerCapacityCmd(opts *options) *cobra.Command {
	var region, zone string
	cmd := &cobra.Command{
		Use: "capacity", Short: "show available capacity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				var nodes []topology.Node
				var zoneID string
				if zone != "" {
					z, err := st.Topology.Store().GetZone(zone)
					if err != nil {
						return err
					}
					zoneID = z.ID
				} else if region != "" {
					reg, err := st.Topology.Store().GetRegion(region)
					if err != nil {
						return err
					}
					zones, _ := st.Topology.Store().ListZones(reg.ID)
					for _, z := range zones {
						ns, _ := st.Topology.Store().ListNodes(z.ID)
						nodes = append(nodes, ns...)
					}
					goto print
				}
				{
					var err error
					nodes, err = st.Topology.Store().ListNodes(zoneID)
					if err != nil {
						return err
					}
				}
			print:
				var totalCPU int
				var totalMem int64
				ready := 0
				for _, n := range nodes {
					if n.Status == topology.StatusReady {
						ready++
						totalCPU += n.CPUCount
						totalMem += n.MemoryBytes
					}
				}
				if opts.json {
					return printJSON(map[string]any{
						"totalNodes": len(nodes), "readyNodes": ready,
						"totalCpu": totalCPU, "totalMemory": totalMem,
					})
				}
				fmt.Printf("Nodes: %d total, %d ready\n", len(nodes), ready)
				fmt.Printf("Total CPU: %s\n", strconv.Itoa(totalCPU))
				fmt.Printf("Total Memory: %s\n", humanSize(totalMem))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "filter by region")
	cmd.Flags().StringVar(&zone, "zone", "", "filter by zone")
	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseLabels(pairs []string) map[string]string {
	out := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

