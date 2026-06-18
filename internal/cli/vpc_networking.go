package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"capper/internal/store"
	"capper/internal/vpc"
)

// ---------------------------------------------------------------------------
// capper sg  (security groups)
// ---------------------------------------------------------------------------

func sgCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "sg", Short: "manage VPC security groups"}
	cmd.AddCommand(
		sgCreateCmd(opts),
		sgListCmd(opts),
		sgDeleteCmd(opts),
		sgRuleCmd(opts),
	)
	return cmd
}

func sgCreateCmd(opts *options) *cobra.Command {
	var vpcID, desc string
	var allowAll bool
	cmd := &cobra.Command{
		Use: "create <name>", Short: "create a security group",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				sg, err := st.VPC.CreateSecurityGroup(vpcID, args[0], desc, !allowAll)
				if err != nil {
					return err
				}
				fmt.Printf("Created security group %s (%s)\n", sg.Name, sg.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	cmd.Flags().StringVar(&desc, "desc", "", "description")
	cmd.Flags().BoolVar(&allowAll, "allow-all", false, "disable default-deny (allow by default)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

func sgListCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "list", Short: "list security groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				sgs, err := st.VPC.ListSecurityGroups(vpcID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(sgs)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tNAME\tVPC\tDEFAULT-DENY")
				for _, sg := range sgs {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", sg.ID, sg.Name, sg.VPCID, sg.DefaultDeny)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "filter by VPC id")
	return cmd
}

func sgDeleteCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "delete <name-or-id>", Short: "delete a security group",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteSecurityGroup(args[0], vpcID)
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

func sgRuleCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "rule", Short: "manage security group rules"}
	cmd.AddCommand(sgRuleAddCmd(opts), sgRuleListCmd(opts), sgRuleDeleteCmd(opts))
	return cmd
}

func sgRuleAddCmd(opts *options) *cobra.Command {
	var proto, cidr, action, dir string
	var fromPort, toPort int
	cmd := &cobra.Command{
		Use: "add <sg-id>", Short: "add a rule to a security group",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				d := vpc.SGRuleDirection(dir)
				if d != vpc.SGIngress && d != vpc.SGEgress {
					return fmt.Errorf("--dir must be ingress or egress")
				}
				rule, err := st.VPC.AddSGRule(args[0], d, proto, cidr, fromPort, toPort, action)
				if err != nil {
					return err
				}
				fmt.Printf("Added rule %s (%s %s %s %d-%d)\n", rule.ID, rule.Direction, rule.Protocol, rule.CIDR, rule.FromPort, rule.ToPort)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "ingress", "direction: ingress or egress")
	cmd.Flags().StringVar(&proto, "proto", "tcp", "protocol: tcp, udp, icmp, all")
	cmd.Flags().StringVar(&cidr, "cidr", "0.0.0.0/0", "CIDR range")
	cmd.Flags().StringVar(&action, "action", "allow", "allow or deny")
	cmd.Flags().IntVar(&fromPort, "from", 0, "from port")
	cmd.Flags().IntVar(&toPort, "to", 0, "to port")
	return cmd
}

func sgRuleListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list <sg-id>", Short: "list rules in a security group",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				rules, err := st.VPC.ListSGRules(args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(rules)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tDIR\tPROTO\tCIDR\tPORTS\tACTION")
				for _, r := range rules {
					ports := fmt.Sprintf("%d-%d", r.FromPort, r.ToPort)
					if r.FromPort == 0 && r.ToPort == 0 {
						ports = "any"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Direction, r.Protocol, r.CIDR, ports, r.Action)
				}
				return tw.Flush()
			})
		},
	}
}

func sgRuleDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete <rule-id>", Short: "delete a security group rule",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteSGRule(args[0])
			})
		},
	}
}

// ---------------------------------------------------------------------------
// capper igw  (internet gateways)
// ---------------------------------------------------------------------------

func igwCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "igw", Short: "manage internet gateways"}
	cmd.AddCommand(igwCreateCmd(opts), igwListCmd(opts), igwDeleteCmd(opts))
	return cmd
}

func igwCreateCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "create <name>", Short: "create an internet gateway",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				igw, err := st.VPC.CreateIGW(vpcID, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("Created internet gateway %s (%s)\n", igw.Name, igw.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

func igwListCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "list", Short: "list internet gateways",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				igws, err := st.VPC.ListIGWs(vpcID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(igws)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tNAME\tVPC\tCREATED")
				for _, igw := range igws {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", igw.ID, igw.Name, igw.VPCID, igw.CreatedAt)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "filter by VPC id")
	return cmd
}

func igwDeleteCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "delete <name-or-id>", Short: "delete an internet gateway",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteIGW(args[0], vpcID)
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

// ---------------------------------------------------------------------------
// capper nat  (NAT gateways)
// ---------------------------------------------------------------------------

func natCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "nat", Short: "manage NAT gateways"}
	cmd.AddCommand(natCreateCmd(opts), natListCmd(opts), natDeleteCmd(opts))
	return cmd
}

func natCreateCmd(opts *options) *cobra.Command {
	var vpcID, subnetID, publicIP string
	cmd := &cobra.Command{
		Use: "create <name>", Short: "create a NAT gateway",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				nat, err := st.VPC.CreateNATGateway(vpcID, subnetID, args[0], publicIP)
				if err != nil {
					return err
				}
				fmt.Printf("Created NAT gateway %s (%s)\n", nat.Name, nat.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	cmd.Flags().StringVar(&subnetID, "subnet", "", "subnet id or name (required)")
	cmd.Flags().StringVar(&publicIP, "public-ip", "", "public IP address")
	_ = cmd.MarkFlagRequired("vpc")
	_ = cmd.MarkFlagRequired("subnet")
	return cmd
}

func natListCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "list", Short: "list NAT gateways",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				nats, err := st.VPC.ListNATGateways(vpcID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(nats)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tNAME\tVPC\tSUBNET\tPUBLIC-IP")
				for _, nat := range nats {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", nat.ID, nat.Name, nat.VPCID, nat.SubnetID, nat.PublicIP)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "filter by VPC id")
	return cmd
}

func natDeleteCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "delete <name-or-id>", Short: "delete a NAT gateway",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteNATGateway(args[0], vpcID)
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

// ---------------------------------------------------------------------------
// capper route-table
// ---------------------------------------------------------------------------

func routeTableCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "route-table", Short: "manage VPC route tables"}
	cmd.AddCommand(
		routeTableCreateCmd(opts),
		routeTableListCmd(opts),
		routeTableDeleteCmd(opts),
		routeAddCmd(opts),
		routeListCmd(opts),
		routeDeleteCmd(opts),
		routeTableAssociateCmd(opts),
	)
	return cmd
}

func routeTableCreateCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "create <name>", Short: "create a route table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				rt, err := st.VPC.CreateRouteTable(vpcID, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("Created route table %s (%s)\n", rt.Name, rt.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

func routeTableListCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "list", Short: "list route tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				rts, err := st.VPC.ListRouteTables(vpcID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(rts)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tNAME\tVPC\tCREATED")
				for _, rt := range rts {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", rt.ID, rt.Name, rt.VPCID, rt.CreatedAt)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "filter by VPC id")
	return cmd
}

func routeTableDeleteCmd(opts *options) *cobra.Command {
	var vpcID string
	cmd := &cobra.Command{
		Use: "delete <name-or-id>", Short: "delete a route table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteRouteTable(args[0], vpcID)
			})
		},
	}
	cmd.Flags().StringVar(&vpcID, "vpc", "", "VPC id or name (required)")
	_ = cmd.MarkFlagRequired("vpc")
	return cmd
}

func routeAddCmd(opts *options) *cobra.Command {
	var dest, targetType, targetID string
	cmd := &cobra.Command{
		Use: "add-route <route-table-id>", Short: "add a route to a route table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r, err := st.VPC.AddRoute(args[0], dest, targetType, targetID)
				if err != nil {
					return err
				}
				fmt.Printf("Added route %s → %s:%s\n", r.DestinationCIDR, r.TargetType, r.TargetID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "destination CIDR (required)")
	cmd.Flags().StringVar(&targetType, "target-type", "local", "target type: igw, nat, local, instance")
	cmd.Flags().StringVar(&targetID, "target-id", "", "target resource id")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}

func routeListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list-routes <route-table-id>", Short: "list routes in a route table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				routes, err := st.VPC.ListRoutes(args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(routes)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tDESTINATION\tTARGET-TYPE\tTARGET")
				for _, r := range routes {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.DestinationCIDR, r.TargetType, r.TargetID)
				}
				return tw.Flush()
			})
		},
	}
}

func routeDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "delete-route <route-id>", Short: "delete a route",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				return st.VPC.DeleteRoute(args[0])
			})
		},
	}
}

func routeTableAssociateCmd(opts *options) *cobra.Command {
	var subnetID string
	cmd := &cobra.Command{
		Use: "associate <route-table-id>", Short: "associate a subnet with a route table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if err := st.VPC.AssociateSubnet(subnetID, args[0]); err != nil {
					return err
				}
				fmt.Printf("Associated subnet %s with route table %s\n", subnetID, args[0])
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&subnetID, "subnet", "", "subnet id (required)")
	_ = cmd.MarkFlagRequired("subnet")
	return cmd
}
