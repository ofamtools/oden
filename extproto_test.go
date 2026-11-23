package oden

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAddExtensionProtocolAndBuiltins(t *testing.T) {
	now := time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)
	manifest := ExtManifest{
		Name:    "o-demo",
		Version: "v2.0.0",
		Schemas: []ExtSchema{
			{
				Kind:           "demo.item",
				Title:          "Item",
				Plural:         "Items",
				SummaryField:   "title",
				DefaultColumns: []string{"title", "status"},
				Fields: []ExtField{
					{Name: "title", Type: "string", Required: true, Description: "Item title"},
					{Name: "status", Type: "string", Enum: []string{"open", "done"}},
				},
			},
		},
		Commands: []ExtCommand{
			{ID: "demo.item.list", Description: "List items"},
		},
	}
	caps := Capabilities{
		Summary:      "Demo extension",
		Capabilities: []string{"catalog", "query"},
	}

	var gotKind string
	var gotArgs []string
	queryFn := func(_ context.Context, kind string, args []string) (ExtQueryResult, error) {
		gotKind = kind
		gotArgs = args
		return ExtQueryResult{
			Items: []ExtRecord{
				{
					Kind:          "demo.item",
					ID:            "i-1",
					Fields:        map[string]any{"title": "Hello", "status": "open"},
					ArchiveStatus: "active",
					StarStatus:    "unstarred",
					CreatedAt:     now,
					UpdatedAt:     now,
				},
			},
		}, nil
	}

	run := func(args ...string) (string, error) {
		var out, errOut bytes.Buffer
		app := New("o-demo", args, WithBuiltins(), WithPath(t.TempDir()), WithStdio(strings.NewReader(""), &out, &errOut))
		app.Cmd("ping", "ping demo", func(c *Ctx) error { return c.Println("pong") })
		app.AddExtension(manifest, caps, queryFn)
		err := app.Run(context.Background())
		return out.String(), err
	}

	// 1. `catalog` prints ExtManifest JSON (overriding builtin catalog).
	out, err := run("catalog")
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	var gotManifest ExtManifest
	if err := json.Unmarshal([]byte(out), &gotManifest); err != nil {
		t.Fatalf("unmarshal catalog: %v\n%s", err, out)
	}
	if gotManifest.Name != "o-demo" || len(gotManifest.Schemas) != 1 || gotManifest.Schemas[0].Kind != "demo.item" {
		t.Fatalf("unexpected manifest: %+v", gotManifest)
	}

	// 2. `query` prints ExtQueryResult JSON with --kind and extra args forwarded.
	out, err = run("query", "--kind", "demo.item", "hello")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if gotKind != "demo.item" || strings.Join(gotArgs, ",") != "hello" {
		t.Fatalf("queryFn got kind=%q args=%v", gotKind, gotArgs)
	}
	var gotRes ExtQueryResult
	if err := json.Unmarshal([]byte(out), &gotRes); err != nil {
		t.Fatalf("unmarshal query: %v\n%s", err, out)
	}
	if gotRes.Total != 1 || len(gotRes.Items) != 1 || gotRes.Items[0].ID != "i-1" {
		t.Fatalf("unexpected query result: %+v", gotRes)
	}

	// 3. `capabilities --json` prints Capabilities JSON with auto-populated commands.
	out, err = run("capabilities", "--json")
	if err != nil {
		t.Fatalf("capabilities --json: %v", err)
	}
	var gotCaps Capabilities
	if err := json.Unmarshal([]byte(out), &gotCaps); err != nil {
		t.Fatalf("unmarshal capabilities: %v\n%s", err, out)
	}
	if gotCaps.Name != "o-demo" || gotCaps.Version != "v2.0.0" || gotCaps.Summary != "Demo extension" {
		t.Fatalf("unexpected capabilities: %+v", gotCaps)
	}
	if len(gotCaps.Commands) == 0 {
		t.Fatalf("expected auto-populated commands in capabilities")
	}

	// 4. `capabilities` without --json prints a human table.
	out, err = run("capabilities")
	if err != nil {
		t.Fatalf("capabilities table: %v", err)
	}
	if !strings.Contains(out, "FIELD") || !strings.Contains(out, "Demo extension") {
		t.Fatalf("unexpected capabilities table:\n%s", out)
	}

	// 5. Builtins (`commands`, `command list`, `help`, `help --all`, `which`, `version`) still work cleanly.
	for _, builtinArgs := range [][]string{
		{"commands"},
		{"command", "list"},
		{"help"},
		{"help", "--all"},
		{"which", "catalog"},
		{"version"},
	} {
		out, err := run(builtinArgs...)
		if err != nil {
			t.Fatalf("builtin %v failed: %v", builtinArgs, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("builtin %v produced empty output", builtinArgs)
		}
	}
}
