---
title: "Adding a CLI command"
description: "Wire a new cobra command into the capper CLI."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Adding a CLI command

The CLI is a [cobra](https://github.com/spf13/cobra) command tree under
`internal/cli`. Each subsystem registers a command group with verbs that call the
controller/SDK.

## Pattern

```go
// internal/cli/<subsystem>.go
func widgetCmd(opts *options) *cobra.Command {
    cmd := &cobra.Command{Use: "widget", Short: "manage widgets"}

    list := &cobra.Command{
        Use:   "list",
        Short: "list widgets",
        RunE: func(cmd *cobra.Command, args []string) error {
            return withController(opts, func(ctrl controller.Controller) error {
                items, err := ctrl.Widgets.List()
                if err != nil {
                    return err
                }
                return printResult(opts, items)   // respects --json
            })
        },
    }
    list.Flags().String("filter", "", "filter widgets")

    cmd.AddCommand(list)
    return cmd
}
```

Register the group on the root command (where the other `*Cmd(opts)` groups are
added).

## Conventions

- Respect the **global flags** (`--store`, `--project`, `--json`, `--runtime`,
  `--debug`); don't redefine them.
- Honor `--json` for machine-readable output.
- Return errors (`RunE`) rather than calling `os.Exit`; the root handles exit
  codes.
- Use `--help`-friendly `Short`/`Long` text — the
  [CLI reference](../reference/cli/capper.md) points readers at `--help`, so keep
  it accurate.
- Keep parity: a CLI verb should have matching API + SDK coverage.

## Validate

```bash
go build ./cmd/capper && ./bin/capper widget --help
go vet ./... && go test ./...
```

## Related

- [Adding a module](adding-a-module.md) · [CLI reference](../reference/cli/capper.md)
