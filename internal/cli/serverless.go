package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"capper/internal/functions"
	"capper/internal/mcpserver"
	"capper/internal/store"
)

// fnCmd implements `capper fn`.
func fnCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "fn", Short: "manage serverless functions"}
	cmd.AddCommand(fnCreateCmd(opts), fnListCmd(opts), fnInvokeCmd(opts), fnDeleteCmd(opts), fnLogsCmd(opts))
	return cmd
}

func fnCreateCmd(opts *options) *cobra.Command {
	var runtime, image string
	var command []string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "create a function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				fn, err := st.Functions.CreateFunction(functions.Function{
					Project: opts.project, Name: args[0], Runtime: runtime, Image: image, Command: command,
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(fn)
				}
				fmt.Printf("Function created: %s (%s)\n", fn.Name, fn.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&runtime, "runtime", "native", "function runtime")
	cmd.Flags().StringVar(&image, "image", "", "function image")
	cmd.Flags().StringArrayVar(&command, "command", nil, "command to execute (repeatable)")
	return cmd
}

func fnListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list functions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				fns, err := st.Functions.ListFunctions(opts.project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(fns)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tRUNTIME\tSTATUS\tID")
				for _, f := range fns {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", f.Name, f.Runtime, f.Status, f.ID)
				}
				return tw.Flush()
			})
		},
	}
}

func fnInvokeCmd(opts *options) *cobra.Command {
	var payload string
	cmd := &cobra.Command{
		Use:   "invoke NAME",
		Short: "invoke a function synchronously",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				fn, err := st.Functions.GetFunctionByName(opts.project, args[0])
				if err != nil {
					return err
				}
				mgr := functions.NewManager(st.Functions, nil)
				res, err := mgr.Invoke(context.Background(), fn, []byte(payload), "", "cli", "manual")
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(res)
				}
				fmt.Printf("status=%s duration=%dms\n%s\n", res.Status, res.DurationMS, res.Output)
				if res.Error != "" {
					fmt.Printf("error: %s\n", res.Error)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&payload, "payload", "", "payload sent to the function on stdin")
	return cmd
}

func fnDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "delete a function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				fn, err := st.Functions.GetFunctionByName(opts.project, args[0])
				if err != nil {
					return err
				}
				if err := st.Functions.DeleteFunction(fn.ID); err != nil {
					return err
				}
				fmt.Printf("Function %q deleted.\n", args[0])
				return nil
			})
		},
	}
}

func fnLogsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "invocations NAME",
		Short: "list recent invocations of a function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				fn, err := st.Functions.GetFunctionByName(opts.project, args[0])
				if err != nil {
					return err
				}
				invs, err := st.Functions.ListInvocations(fn.ID, 50)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(invs)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "STARTED\tSTATUS\tDURATION_MS\tSOURCE")
				for _, i := range invs {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", i.StartedAt, i.Status, i.DurationMS, i.Source)
				}
				return tw.Flush()
			})
		},
	}
}

// mcpCmd implements `capper mcp`.
func mcpCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "manage MCP servers"}
	cmd.AddCommand(mcpDeployCmd(opts), mcpListCmd(opts), mcpToolsCmd(opts), mcpApprovalsCmd(opts))
	return cmd
}

func mcpDeployCmd(opts *options) *cobra.Command {
	var runtime, approvalPolicy string
	cmd := &cobra.Command{
		Use:   "deploy NAME",
		Short: "deploy an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				srv, err := st.MCPServers.CreateServer(mcpserver.Server{
					Project: opts.project, Name: args[0], Runtime: runtime, ApprovalPolicy: approvalPolicy,
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(srv)
				}
				fmt.Printf("MCP server deployed: %s (%s)\n", srv.Name, srv.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&runtime, "runtime", "mcp-go", "MCP server runtime")
	cmd.Flags().StringVar(&approvalPolicy, "approval-policy", "dangerous-only", "none|dangerous-only|all")
	return cmd
}

func mcpListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				servers, err := st.MCPServers.ListServers(opts.project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(servers)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tRUNTIME\tAPPROVAL\tSTATUS\tID")
				for _, s := range servers {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Runtime, s.ApprovalPolicy, s.Status, s.ID)
				}
				return tw.Flush()
			})
		},
	}
}

func mcpToolsCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "tools", Short: "manage MCP server tools"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list SERVER",
		Short: "list a server's tools",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				srv, err := findMCPServer(st, opts.project, args[0])
				if err != nil {
					return err
				}
				tools, err := st.MCPServers.ListTools(srv.ID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(tools)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tIAM_ACTION\tREADONLY\tDANGEROUS\tENABLED")
				for _, t := range tools {
					fmt.Fprintf(tw, "%s\t%s\t%t\t%t\t%t\n", t.Name, t.IAMAction, t.ReadOnly, t.Dangerous, t.Enabled)
				}
				return tw.Flush()
			})
		},
	})
	return cmd
}

func mcpApprovalsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "approvals",
		Short: "list pending MCP tool approvals",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				approvals, err := st.MCPServers.ListApprovals(mcpserver.ApprovalPending)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(approvals)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tTOOL\tPRINCIPAL\tCREATED")
				for _, a := range approvals {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ID, a.ToolName, a.Principal, a.CreatedAt)
				}
				return tw.Flush()
			})
		},
	}
}

// findMCPServer resolves an MCP server by name or ID within a project.
func findMCPServer(st *store.Store, project, nameOrID string) (mcpserver.Server, error) {
	if strings.HasPrefix(nameOrID, "mcp_") {
		return st.MCPServers.GetServer(nameOrID)
	}
	servers, err := st.MCPServers.ListServers(project)
	if err != nil {
		return mcpserver.Server{}, err
	}
	for _, s := range servers {
		if s.Name == nameOrID {
			return s, nil
		}
	}
	return mcpserver.Server{}, fmt.Errorf("mcp server %q not found", nameOrID)
}
