package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	csdserver "capper/internal/csd/server"
)

func volumeCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "manage CSD shared volumes",
	}
	cmd.AddCommand(
		volumeCreateCmd(opts),
		volumeListCmd(opts),
		volumeInspectCmd(opts),
		volumeDeleteCmd(opts),
		volumeAttachCmd(opts),
		volumeDetachCmd(opts),
		volumeAttachmentsCmd(opts),
		volumeSnapshotCmd(opts),
		volumeSnapshotsCmd(opts),
		volumeSnapshotDeleteCmd(opts),
		volumeLeasesCmd(opts),
		volumeRevokeLeasesCmd(opts),
		volumeRepairCmd(opts),
	)
	return cmd
}

func volumeCreateCmd(opts *options) *cobra.Command {
	var mode string
	var sizeStr string
	var replicas int
	var encrypted bool
	var encKey string
	var storageClass string

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "create a new CSD shared volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sizeBytes, err := parseSize(sizeStr)
			if err != nil {
				return fmt.Errorf("invalid --size: %w", err)
			}
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:create", "project:"+opts.project); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Create(context.Background(), csdserver.CreateVolumeOpts{
					Project:      opts.project,
					Name:         args[0],
					Mode:         mode,
					SizeBytes:    sizeBytes,
					StorageClass: storageClass,
					ReplicaCount: replicas,
					Encrypted:    encrypted,
					EncKeyID:     encKey,
				})
				if err != nil {
					return err
				}
				ac.RecordEvent("csd_volume", v.ID, "csd.volume.created", opts.project,
					map[string]any{"name": v.Name, "mode": v.Mode})
				if opts.json {
					return printJSON(v)
				}
				fmt.Printf("Created volume %q\nID:       %s\nMode:     %s\nSize:     %s\nStatus:   %s\n",
					v.Name, v.ID, v.Mode, formatBytes(v.SizeBytes), v.Status)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "shared-fs", "volume mode: shared-fs, single-writer, shared-block")
	cmd.Flags().StringVar(&sizeStr, "size", "1G", "volume size (e.g. 500M, 10G)")
	cmd.Flags().IntVar(&replicas, "replicas", 1, "replica count")
	cmd.Flags().BoolVar(&encrypted, "encrypted", false, "encrypt at rest")
	cmd.Flags().StringVar(&encKey, "enc-key", "", "KMS key name for encryption")
	cmd.Flags().StringVar(&storageClass, "class", "local", "storage class")
	return cmd
}

func volumeListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list CSD volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:list", "project:"+opts.project); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				vols, err := mgr.List(context.Background(), opts.project)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(vols)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tID\tMODE\tSIZE\tSTATUS\tCREATED")
				for _, v := range vols {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						v.Name, v.ID, v.Mode, formatBytes(v.SizeBytes), v.Status, shortTime(v.CreatedAt))
				}
				return tw.Flush()
			})
		},
	}
}

func volumeInspectCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect NAME",
		Short: "show volume details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:inspect", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				if opts.json {
					return printJSON(v)
				}
				fmt.Printf("Name:      %s\nID:        %s\nProject:   %s\nMode:      %s\nSize:      %s\nUsed:      %s\nStatus:    %s\nReplicas:  %d\nEpoch:     %d\nEncrypted: %v\nCreated:   %s\n",
					v.Name, v.ID, v.Project, v.Mode,
					formatBytes(v.SizeBytes), formatBytes(v.UsedBytes),
					v.Status, v.ReplicaCount, v.Epoch, v.Encrypted,
					shortTime(v.CreatedAt))
				return nil
			})
		},
	}
}

func volumeDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "delete a CSD volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:delete", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				if err := mgr.Delete(context.Background(), args[0], opts.project); err != nil {
					return err
				}
				ac.RecordEvent("csd_volume", args[0], "csd.volume.deleted", opts.project, nil)
				fmt.Printf("Deleted volume %q\n", args[0])
				return nil
			})
		},
	}
}

func volumeAttachCmd(opts *options) *cobra.Command {
	var mountPath string
	var accessMode string

	cmd := &cobra.Command{
		Use:   "attach VOLUME INSTANCE",
		Short: "attach a volume to an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:attach", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				a, err := mgr.Attach(context.Background(), csdserver.AttachOpts{
					VolumeID:   v.ID,
					InstanceID: args[1],
					MountPath:  mountPath,
					AccessMode: accessMode,
				})
				if err != nil {
					return err
				}
				ac.RecordEvent("csd_volume", v.ID, "csd.volume.attached", opts.project,
					map[string]any{"instanceId": args[1], "mountPath": mountPath})
				if opts.json {
					return printJSON(a)
				}
				fmt.Printf("Attached volume %q to instance %q\nAttachment ID: %s\nMount path:    %s\nAccess:        %s\n",
					args[0], args[1], a.ID, a.MountPath, a.AccessMode)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&mountPath, "mount", "/mnt/csd", "mount path inside the instance")
	cmd.Flags().StringVar(&accessMode, "access", "rw", "access mode: rw, ro")
	return cmd
}

func volumeDetachCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "detach VOLUME INSTANCE",
		Short: "detach a volume from an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:detach", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				if err := mgr.Detach(context.Background(), args[0], args[1]); err != nil {
					return err
				}
				fmt.Printf("Detached volume %q from instance %q\n", args[0], args[1])
				return nil
			})
		},
	}
}

func volumeAttachmentsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "attachments VOLUME",
		Short: "list attachments for a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:inspect", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				atts, err := mgr.ListAttachments(context.Background(), args[0])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(atts)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tINSTANCE\tMOUNT\tACCESS\tSTATUS\tATTACHED")
				for _, a := range atts {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						a.ID, a.InstanceID, a.MountPath, a.AccessMode, a.Status, shortTime(a.AttachedAt))
				}
				return tw.Flush()
			})
		},
	}
}

func volumeSnapshotCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot VOLUME NAME",
		Short: "create a snapshot of a CSD volume",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:snapshot", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				snap, err := csdserver.NewSnapshotManager(ac.Store.CSD).Create(context.Background(), v.ID, args[1])
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(snap)
				}
				fmt.Printf("Created snapshot %q\nID:      %s\nVolume:  %s\nSize:    %s\nVersion: %d\n",
					snap.Name, snap.ID, snap.VolumeID, formatBytes(snap.SizeBytes), snap.RootVersion)
				return nil
			})
		},
	}
}

func volumeSnapshotsCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshots VOLUME",
		Short: "list snapshots for a CSD volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:inspect", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				snaps, err := csdserver.NewSnapshotManager(ac.Store.CSD).List(context.Background(), v.ID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(snaps)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tID\tSIZE\tVERSION\tSTATUS\tCREATED")
				for _, s := range snaps {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
						s.Name, s.ID, formatBytes(s.SizeBytes), s.RootVersion, s.Status, shortTime(s.CreatedAt))
				}
				return tw.Flush()
			})
		},
	}
}

func volumeSnapshotDeleteCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot-delete VOLUME SNAPSHOT",
		Short: "delete a CSD volume snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:snapshot", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				if err := csdserver.NewSnapshotManager(ac.Store.CSD).Delete(context.Background(), v.ID, args[1]); err != nil {
					return err
				}
				fmt.Printf("Deleted snapshot %q from volume %q\n", args[1], args[0])
				return nil
			})
		},
	}
}

func volumeLeasesCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "leases VOLUME",
		Short: "list active leases for a CSD volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:inspect", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				leases, err := ac.Store.CSD.Leases.ForVolume(v.ID)
				if err != nil {
					return err
				}
				if opts.json {
					return printJSON(leases)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tCLIENT\tINODE\tTYPE\tEXPIRES")
				for _, l := range leases {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						l.ID, l.ClientID, l.InodeID, l.LeaseType, l.ExpiresAt.Format("2006-01-02T15:04:05Z"))
				}
				return tw.Flush()
			})
		},
	}
}

func volumeRevokeLeasesCmd(opts *options) *cobra.Command {
	var clientID string
	cmd := &cobra.Command{
		Use:   "revoke-lease VOLUME",
		Short: "revoke all leases held by a client on a CSD volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clientID == "" {
				return fmt.Errorf("--client is required")
			}
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:admin", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				n, err := ac.Store.CSD.Leases.DeleteForClient(v.ID, clientID)
				if err != nil {
					return err
				}
				fmt.Printf("Revoked %d lease(s) for client %q on volume %q\n", n, clientID, args[0])
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&clientID, "client", "", "client ID whose leases to revoke")
	return cmd
}

func volumeRepairCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "repair VOLUME",
		Short: "replay journal and reset volume status to available",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withIAM(opts, func(ac *iamCtx) error {
				if err := ac.Authorize("csd:admin", "csd/"+args[0]); err != nil {
					return err
				}
				mgr := csdserver.NewVolumeManager(ac.Store.CSD)
				v, err := mgr.Get(context.Background(), args[0], opts.project)
				if err != nil {
					return fmt.Errorf("volume %q not found", args[0])
				}
				store := ac.Store.CSD
				jm := csdserver.NewJournalManager(store)
				mm := csdserver.NewMetadataManager(store, jm)
				if err := jm.Replay(context.Background(), mm, v.ID); err != nil {
					return fmt.Errorf("journal replay: %w", err)
				}
				_ = store.Volumes.UpdateStatus(v.ID, "available")
				fmt.Printf("Repaired volume %q: journal replayed, status reset to available\n", args[0])
				return nil
			})
		},
	}
}

// formatBytes converts a byte count into a human-readable string.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
