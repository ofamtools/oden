package oden

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFlagsBasicAndTypes(t *testing.T) {
	params := []Parameter{
		{Name: "title", Short: "t", Type: "string", Required: true, Description: "Item title"},
		{Name: "count", Short: "n", Type: "int", Default: "3"},
		{Name: "ratio", Type: "float", Default: "1.5"},
		{Name: "verbose", Short: "v", Type: "bool"},
		{Name: "status", Flag: "state", Type: "string", Enum: []string{"open", "done"}, Default: "open"},
		{Name: "field", Short: "f", Type: "string", Repeatable: true},
		{Name: "tag", Type: "list"},
	}

	p, err := ParseFlags([]string{
		"pos1",
		"-t", "hello",
		"--count=7",
		"--ratio", "2.25",
		"-v",
		"--state=done",
		"--field", "a=1",
		"-f", "b=2",
		"--tag", "x",
		"--tag=y",
		"--json",
		"--",
		"--not-a-flag",
		"pos2",
	}, params)
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	if got := p.String("title"); got != "hello" {
		t.Fatalf("title = %q", got)
	}
	if !p.Has("title") || !p.Has("t") {
		t.Fatalf("expected Has(title) and Has(t) to be true")
	}
	if got := p.Int("count"); got != 7 {
		t.Fatalf("count = %d", got)
	}
	if got := p.Float("ratio"); got != 2.25 {
		t.Fatalf("ratio = %v", got)
	}
	if !p.Bool("verbose") {
		t.Fatalf("expected verbose=true")
	}
	if got := p.String("status"); got != "done" {
		t.Fatalf("status = %q", got)
	}
	if got := p.String("state"); got != "done" {
		t.Fatalf("state alias = %q", got)
	}
	if got := strings.Join(p.Strings("field"), ","); got != "a=1,b=2" {
		t.Fatalf("field = %q", got)
	}
	if got := strings.Join(p.Strings("tag"), ","); got != "x,y" {
		t.Fatalf("tag = %q", got)
	}
	if got := p.Format(); got != "json" {
		t.Fatalf("Format() = %q, want json", got)
	}
	if got := strings.Join(p.Positional(), ","); got != "pos1,--not-a-flag,pos2" {
		t.Fatalf("Positional() = %q", got)
	}
	if got := p.Arg(1, "fallback"); got != "--not-a-flag" {
		t.Fatalf("Arg(1) = %q", got)
	}
	if got := p.Arg(9, "fallback"); got != "fallback" {
		t.Fatalf("Arg(9) = %q", got)
	}
}

func TestParseFlagsDefaultsAndBoolFalse(t *testing.T) {
	params := []Parameter{
		{Name: "limit", Type: "int", Default: "10"},
		{Name: "flag", Type: "bool", Default: "true"},
	}

	p, err := ParseFlags([]string{"--flag=false", "--format", "table"}, params)
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if p.Has("limit") {
		t.Fatalf("limit should not be marked Has when using default")
	}
	if got := p.Int("limit"); got != 10 {
		t.Fatalf("limit default = %d, want 10", got)
	}
	if p.Bool("flag") {
		t.Fatalf("expected flag=false")
	}
	if got := p.Format(); got != "table" {
		t.Fatalf("Format() = %q, want table", got)
	}
}

func TestParseFlagsErrors(t *testing.T) {
	params := []Parameter{
		{Name: "name", Type: "string", Required: true},
		{Name: "count", Type: "int"},
		{Name: "rate", Type: "float"},
		{Name: "flag", Type: "bool"},
		{Name: "mode", Type: "string", Enum: []string{"fast", "slow"}},
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing required", nil, "missing required flag: --name"},
		{"unknown long", []string{"--name", "a", "--bogus"}, "unknown flag: --bogus"},
		{"unknown short", []string{"--name", "a", "-z"}, "unknown flag: -z"},
		{"missing value", []string{"--name"}, "requires a value"},
		{"bad int", []string{"--name", "a", "--count", "nope"}, "invalid integer"},
		{"bad float", []string{"--name", "a", "--rate", "nope"}, "invalid float"},
		{"bad bool", []string{"--name", "a", "--flag=maybe"}, "invalid boolean"},
		{"bad enum", []string{"--name", "a", "--mode", "turbo"}, "invalid value"},
		{"bad format", []string{"--name", "a", "--format", "yaml"}, "invalid value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFlags(tc.args, params)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got err %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCtxParseHelpAndMachineMetadata(t *testing.T) {
	params := []Parameter{
		{Name: "status", Short: "s", Type: "string", Default: "open", Enum: []string{"open", "done"}, Description: "Filter by status"},
		{Name: "tag", Type: "list", Repeatable: true, Description: "Filter by tag"},
	}

	var gotStatus string
	var gotTags []string
	var out, errOut bytes.Buffer

	makeApp := func(args []string) *App {
		out.Reset()
		errOut.Reset()
		app := New("demo", args, WithBuiltins(), WithPath(t.TempDir()), WithStdio(strings.NewReader(""), &out, &errOut))
		app.Add(Command{
			Name:       "list",
			Summary:    "List items",
			Usage:      "[--status STATUS] [--tag TAG...]",
			Parameters: params,
			Handle: func(c *Ctx) error {
				p, err := c.Parse(params)
				if err != nil {
					return err
				}
				gotStatus = p.String("status")
				gotTags = p.Strings("tag")
				return c.Println(gotStatus)
			},
		})
		return app
	}

	// 1. Ctx.Parse in handler.
	if err := makeApp([]string{"list", "-s", "done", "--tag", "a", "--tag", "b"}).Run(context.Background()); err != nil {
		t.Fatalf("list run: %v", err)
	}
	if gotStatus != "done" || strings.Join(gotTags, ",") != "a,b" {
		t.Fatalf("got status=%q tags=%v", gotStatus, gotTags)
	}

	// 2. Per-command help renders flag metadata.
	if err := makeApp([]string{"help", "list"}).Run(context.Background()); err != nil {
		t.Fatalf("help list: %v", err)
	}
	for _, want := range []string{"Flags:", "-s, --status <string> (optional)", "default: open", "choices: open, done", "--tag <list> (optional, repeatable)"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q in:\n%s", want, out.String())
		}
	}

	// 3. Machine `commands` JSON includes full parameter metadata.
	if err := makeApp([]string{"commands"}).Run(context.Background()); err != nil {
		t.Fatalf("commands: %v", err)
	}
	var entries []Entry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal commands: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name == "list" {
			found = true
			if len(e.Parameters) != 2 || e.Parameters[0].Short != "s" || e.Parameters[0].Default != "open" {
				t.Fatalf("unexpected entry parameters: %+v", e.Parameters)
			}
		}
	}
	if !found {
		t.Fatalf("list entry missing from commands JSON")
	}

	// 4. Machine `command show list` JSON includes parameters too.
	if err := makeApp([]string{"command", "show", "list"}).Run(context.Background()); err != nil {
		t.Fatalf("command show list: %v", err)
	}
	var row struct {
		ID   string      `json:"id"`
		Data CommandData `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &row); err != nil {
		t.Fatalf("unmarshal command show: %v", err)
	}
	if len(row.Data.Parameters) != 2 || row.Data.Parameters[1].Repeatable != true {
		t.Fatalf("unexpected command show parameters: %+v", row.Data.Parameters)
	}
}
