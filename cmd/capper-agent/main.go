package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"capper/internal/agent"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:          "capper-agent",
		Short:        "Capper node agent",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := agent.LoadConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v — using defaults\n", err)
				cfg = agent.DefaultConfig()
			}

			ctx, cancel := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer cancel()

			a := agent.New(cfg)
			return a.Run(ctx)
		},
	}

	root.Flags().StringVar(&configPath, "config", "/etc/capper/agent.yaml", "path to agent config")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
