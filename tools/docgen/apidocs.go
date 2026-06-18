package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// methodPathRe validates a "METHOD /api/v1/..." route string. Go 1.22+ mux
// patterns are "METHOD PATH"; we only document the versioned API surface.
var methodPathRe = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|HEAD)\s+(/api/v1/\S*)$`)

type apiRoute struct {
	method, path string
}

// cmdAPIDocs parses internal/api with the Go AST and collects every route string
// passed to a `.HandleFunc(...)` / `.Handle(...)` call, then writes a complete,
// grouped route reference to docs/src/reference/api/routes.md. Using the AST
// (rather than a raw text scan) means comments and unrelated string literals
// can't produce phantom routes.
func cmdAPIDocs() error {
	dir := filepath.Join("internal", "api")
	routes, err := extractAPIRoutes(dir)
	if err != nil {
		return err
	}

	groups := map[string][]apiRoute{}
	for _, r := range routes {
		g := groupOf(r.path)
		groups[g] = append(groups[g], r)
	}

	var b strings.Builder
	b.WriteString(`---
title: "API reference — all routes"
description: "Every /api/v1 route, grouped by resource. Generated from source."
owner: "docs"
status: "stable"
reviewed: "2026-06-16"
outputs:
  - markdown
  - web
  - pdf
---

# API reference — all routes

`)
	b.WriteString("> Generated from `internal/api` route registrations by `make docs-api`. Do not edit by hand.\n\n")
	b.WriteString("All routes are under `/api/v1` and require [authentication](overview.md) unless listed as public there. ")
	b.WriteString("Responses use the [standard envelope](overview.md#response-envelope). ")
	fmt.Fprintf(&b, "This deployment registers **%d** routes across **%d** groups.\n\n", len(routes), len(groups))

	names := make([]string, 0, len(groups))
	for k := range groups {
		names = append(names, k)
	}
	sort.Strings(names)

	b.WriteString("## Groups\n\n")
	for _, n := range names {
		fmt.Fprintf(&b, "- [`%s`](#%s) — %d routes\n", n, n, len(groups[n]))
	}
	b.WriteString("\n")

	for _, n := range names {
		rs := groups[n]
		sort.Slice(rs, func(i, j int) bool {
			if rs[i].path != rs[j].path {
				return rs[i].path < rs[j].path
			}
			return rs[i].method < rs[j].method
		})
		fmt.Fprintf(&b, "## %s\n\n", n)
		b.WriteString("| Method | Path |\n| --- | --- |\n")
		for _, r := range rs {
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", r.method, r.path)
		}
		b.WriteString("\n")
	}

	out := filepath.Join(srcDir, "reference", "api", "routes.md")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("docs-api: wrote %s (%d routes, %d groups)\n", out, len(routes), len(groups))
	return nil
}

// extractAPIRoutes walks every non-test .go file under dir and returns the unique
// routes registered via `.HandleFunc("METHOD /path", …)` / `.Handle("…", …)`.
func extractAPIRoutes(dir string) ([]apiRoute, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var routes []apiRoute
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			m := methodPathRe.FindStringSubmatch(strings.TrimSpace(s))
			if m == nil {
				return true
			}
			key := m[1] + " " + m[2]
			if seen[key] {
				return true
			}
			seen[key] = true
			routes = append(routes, apiRoute{method: m[1], path: m[2]})
			return true
		})
	}
	return routes, nil
}

// groupOf returns the first path segment after /api/v1/ (e.g. "instances").
func groupOf(path string) string {
	rest := strings.TrimPrefix(path, "/api/v1/")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "root"
	}
	return rest
}
