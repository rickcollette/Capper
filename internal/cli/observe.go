package cli

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"capper/internal/resourcemon"
	"capper/internal/store"
)

// resourcesCmd implements `capper resources`.
func resourcesCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "resources", Short: "unified resource inventory (capper-observe)"}
	cmd.AddCommand(resourcesListCmd(opts), resourcesGetCmd(opts), resourcesSyncCmd(opts))
	return cmd
}

func resourcesListCmd(opts *options) *cobra.Command {
	var rtype, project, status, health string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list resources in the inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				items, err := st.ResourceMon.ListResources(resourcemon.ResourceFilter{
					ResourceType: rtype, Project: project, Status: status, Health: health,
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(items)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TYPE\tNAME\tPROJECT\tSTATUS\tHEALTH\tNODE")
				for _, r := range items {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						r.ResourceType, r.Name, r.Project, r.Status, r.Health, r.NodeID)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&rtype, "type", "", "filter by resource type")
	cmd.Flags().StringVar(&project, "project", "", "filter by project")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&health, "health", "", "filter by health")
	return cmd
}

func resourcesGetCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "get ID",
		Short: "show a resource and its latest config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				r, err := st.ResourceMon.GetResource(args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(r)
				}
				fmt.Printf("ID:       %s\nType:     %s\nName:     %s\nProject:  %s\nStatus:   %s\nHealth:   %s\n",
					r.ID, r.ResourceType, r.Name, r.Project, r.Status, r.Health)
				if cfg, err := st.ResourceMon.LatestConfig(r.ID); err == nil {
					fmt.Printf("Config v%d drift=%s\n", cfg.Version, cfg.DriftStatus)
				}
				return nil
			})
		},
	}
}

func resourcesSyncCmd(opts *options) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "project live resources into the inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if project == "" {
					project = opts.project
				}
				results, err := st.SyncResourceMonitor(project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(results)
				}
				for _, r := range results {
					fmt.Printf("%-16s upserted=%d removed=%d\n", r.ResourceType, r.Upserted, r.Removed)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project to sync (default: --project)")
	return cmd
}

// metricsCmd implements `capper metrics`.
func metricsCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "metrics", Short: "resource metrics"}
	cmd.AddCommand(metricsQueryCmd(opts), metricsIngestCmd(opts))
	return cmd
}

func metricsQueryCmd(opts *options) *cobra.Command {
	var rtype, rid, metric, rng string
	cmd := &cobra.Command{
		Use:   "query",
		Short: "query metric samples for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				q := resourcemon.MetricQuery{ResourceType: rtype, ResourceID: rid, MetricName: metric}
				if rng != "" {
					if d, err := time.ParseDuration(rng); err == nil {
						q.Since = time.Now().UTC().Add(-d).Format(time.RFC3339)
					}
				}
				samples, err := st.ResourceMon.QuerySamples(q)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(samples)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TIME\tVALUE\tUNIT")
				for _, m := range samples {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", m.SampledAt, strconv.FormatFloat(m.Value, 'f', -1, 64), m.Unit)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&rtype, "resource-type", "", "resource type (required)")
	cmd.Flags().StringVar(&rid, "resource-id", "", "resource id (required)")
	cmd.Flags().StringVar(&metric, "metric", "", "metric name (required)")
	cmd.Flags().StringVar(&rng, "range", "1h", "time range (e.g. 1h, 24h)")
	_ = cmd.MarkFlagRequired("resource-type")
	_ = cmd.MarkFlagRequired("resource-id")
	_ = cmd.MarkFlagRequired("metric")
	return cmd
}

func metricsIngestCmd(opts *options) *cobra.Command {
	var rtype, rid, metric, unit string
	var value float64
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "record a single metric sample",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if err := st.ResourceMon.InsertSample(resourcemon.MetricSample{
					ResourceType: rtype, ResourceID: rid, MetricName: metric, Value: value, Unit: unit,
				}); err != nil {
					return err
				}
				fmt.Printf("recorded %s=%v for %s/%s\n", metric, value, rtype, rid)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&rtype, "resource-type", "", "resource type")
	cmd.Flags().StringVar(&rid, "resource-id", "", "resource id")
	cmd.Flags().StringVar(&metric, "metric", "", "metric name")
	cmd.Flags().Float64Var(&value, "value", 0, "metric value")
	cmd.Flags().StringVar(&unit, "unit", "", "unit")
	return cmd
}

// configDriftCmd implements `capper config drift`.
func configCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "resource configuration and drift"}
	drift := &cobra.Command{Use: "drift", Short: "configuration drift"}
	drift.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "list resources whose observed config has drifted",
			RunE: func(cmd *cobra.Command, args []string) error {
				return withStore(opts, func(st *store.Store) error {
					drifted, err := st.ResourceMon.ListDrifted()
					if err != nil {
						return err
					}
					if opts.json {
						return printJSON(drifted)
					}
					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "RESOURCE_ID\tVERSION\tREASON")
					for _, c := range drifted {
						fmt.Fprintf(tw, "%s\tv%d\t%s\n", c.ResourceID, c.Version, c.DriftReason)
					}
					return tw.Flush()
				})
			},
		},
		&cobra.Command{
			Use:   "repair RESOURCE_ID",
			Short: "repair drift by resetting the baseline to desired config",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withStore(opts, func(st *store.Store) error {
					cfg, err := st.ResourceMon.RepairDrift(args[0], "cli")
					if err != nil {
						return err
					}
					fmt.Printf("repaired %s → config v%d (drift=%s)\n", args[0], cfg.Version, cfg.DriftStatus)
					return nil
				})
			},
		},
	)
	cmd.AddCommand(drift)
	return cmd
}

// alertsObserveCmd implements `capper alerts` for the resource monitor.
func alertsObserveCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "alerts", Short: "resource monitor alerts"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "list alerts",
			RunE: func(cmd *cobra.Command, args []string) error {
				return withStore(opts, func(st *store.Store) error {
					alerts, err := st.ResourceMon.ListAlerts("", "", 0)
					if err != nil {
						return err
					}
					if opts.json {
						return printJSON(alerts)
					}
					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "SEVERITY\tSTATUS\tRESOURCE\tTITLE")
					for _, a := range alerts {
						fmt.Fprintf(tw, "%s\t%s\t%s/%s\t%s\n", a.Severity, a.Status, a.ResourceType, a.ResourceID, a.Title)
					}
					return tw.Flush()
				})
			},
		},
		&cobra.Command{
			Use:   "rules",
			Short: "list alert rules",
			RunE: func(cmd *cobra.Command, args []string) error {
				return withStore(opts, func(st *store.Store) error {
					rules, err := st.ResourceMon.ListAlertRules("")
					if err != nil {
						return err
					}
					return printJSON(rules)
				})
			},
		},
	)
	return cmd
}
