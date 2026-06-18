package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"capper/internal/hoststorage"
	"capper/internal/store"
)

// hostStorageCmd implements `capper host-storage` — admin management of the
// host's physical disks as allocatable capacity pools.
func hostStorageCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "host-storage", Short: "manage host physical disks and capacity pools"}
	cmd.AddCommand(
		hostStorageDisksCmd(opts),
		hostStoragePoolCmd(opts),
	)
	return cmd
}

func hostStorageDisksCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "disks",
		Short: "list discovered host disks and their allocation state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				disks, err := hoststorage.NewManager(st.HostStorage).Disks()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(disks)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DEVICE\tSIZE\tTYPE\tSTATE\tMOUNT\tMODEL")
				for _, d := range disks {
					kind := "SSD"
					if d.Rotational {
						kind = "HDD"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", d.Path, formatBytes(d.SizeBytes), kind, d.State, d.Mountpoint, d.Model)
				}
				return tw.Flush()
			})
		},
	}
}

func hostStoragePoolCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "pool", Short: "manage storage pools"}
	cmd.AddCommand(hostStoragePoolCreateCmd(opts), hostStoragePoolListCmd(opts), hostStoragePoolDeleteCmd(opts))
	return cmd
}

func hostStoragePoolCreateCmd(opts *options) *cobra.Command {
	var backend, mountpoint, device, vg string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "register a storage pool (directory over a mount, or an LVM volume group)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				pool, err := hoststorage.NewManager(st.HostStorage).CreatePool(hoststorage.CreatePoolOptions{
					Name: args[0], Backend: backend, Mountpoint: mountpoint, Device: device, VGName: vg,
				})
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(pool)
				}
				where := pool.Mountpoint
				if pool.Backend == hoststorage.BackendLVM {
					where = "vg:" + pool.VGName
				}
				fmt.Printf("Pool %q (%s) created over %s (%s capacity)\n", pool.Name, pool.Backend, where, formatBytes(pool.TotalBytes))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&backend, "backend", "directory", "pool backend: directory | lvm")
	cmd.Flags().StringVar(&mountpoint, "mountpoint", "", "directory backend: mounted path the pool draws from")
	cmd.Flags().StringVar(&device, "device", "", "directory backend: backing device path (for display)")
	cmd.Flags().StringVar(&vg, "vg", "", "lvm backend: volume group name")
	return cmd
}

func hostStoragePoolListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list storage pools with capacity usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				pools, err := hoststorage.NewManager(st.HostStorage).ListPools()
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(pools)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tMOUNT\tTOTAL\tALLOCATED\tAVAILABLE")
				for _, p := range pools {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Mountpoint,
						formatBytes(p.TotalBytes), formatBytes(p.AllocatedBytes), formatBytes(p.AvailableBytes))
				}
				return tw.Flush()
			})
		},
	}
}

func hostStoragePoolDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "delete an (empty) storage pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(opts, func(st *store.Store) error {
				if err := hoststorage.NewManager(st.HostStorage).DeletePool(args[0]); err != nil {
					return err
				}
				fmt.Printf("Pool %q deleted.\n", args[0])
				return nil
			})
		},
	}
}
