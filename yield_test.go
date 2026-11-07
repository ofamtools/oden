package oden

import (
	"context"
	"strings"
	"testing"
)

func yieldApp(t *testing.T, claim func([]string) bool) (*App, *int, *strings.Builder) {
	t.Helper()
	runs := 0
	app, _, _ := testApp(t, fakeFS{}, "", WithBuiltins(), WithClaims(claim))
	app.Default = func(_ context.Context, args []string) error { runs++; return nil }
	out := &strings.Builder{}
	app.Stdout = out
	return app, &runs, out
}

func TestYieldingBuiltinGivesWayWhenClaimed(t *testing.T) {
	for _, w := range []string{"state", "catalog", "commands", "command"} {
		app, runs, out := yieldApp(t, func([]string) bool { return true })
		app.Args = []string{w, "x"}
		if err := app.Run(context.Background()); err != nil || *runs != 1 || out.Len() != 0 {
			t.Fatalf("%s: err %v runs %d out %q", w, err, *runs, out)
		}
	}
	app, runs, _ := yieldApp(t, func(args []string) bool { return args[0] == "state" })
	var got []string
	app.Default = func(_ context.Context, args []string) error { *runs++; got = args; return nil }
	app.Args = []string{"state", "--json"}
	if err := app.Run(context.Background()); err != nil || *runs != 1 || strings.Join(got, " ") != "state --json" {
		t.Fatalf("err %v runs %d got %v", err, *runs, got)
	}
	app.Args = []string{"catalog"} // not claimed: the builtin runs
	if err := app.Run(context.Background()); err != nil || *runs != 1 {
		t.Fatalf("catalog: err %v runs %d", err, *runs)
	}
}

func TestYieldingBuiltinRunsWhenNotClaimed(t *testing.T) {
	app, runs, out := yieldApp(t, func([]string) bool { return false })
	app.Args = []string{"state"}
	if err := app.Run(context.Background()); err != nil || *runs != 0 || out.Len() == 0 {
		t.Fatalf("err %v runs %d out %q", err, *runs, out)
	}
	app, runs, out = yieldApp(t, nil)
	app.Args = []string{"catalog"}
	if err := app.Run(context.Background()); err != nil || *runs != 0 || out.Len() == 0 {
		t.Fatalf("no claims: err %v runs %d", err, *runs)
	}
}

func TestHumanBuiltinsAndAppCommandsNeverYield(t *testing.T) {
	app, runs, out := yieldApp(t, func([]string) bool { return true })
	for _, w := range []string{"help", "version", "info"} {
		out.Reset()
		app.Args = []string{w}
		if err := app.Run(context.Background()); err != nil || out.Len() == 0 {
			t.Fatalf("%s: err %v out %q", w, err, out)
		}
	}
	if *runs != 0 {
		t.Fatalf("human builtin yielded: %d", *runs)
	}
	ran := false
	app.Add(Command{Name: "state", Run: func(context.Context, []string) error { ran = true; return nil }})
	app.Args = []string{"state"}
	if err := app.Run(context.Background()); err != nil || !ran || *runs != 0 {
		t.Fatalf("err %v ran %v runs %d", err, ran, *runs)
	}
}

func TestYieldOnlyOnExactWord(t *testing.T) {
	app, runs, out := yieldApp(t, func([]string) bool { return true })
	app.Args = []string{"stat"} // unique prefix of state
	if err := app.Run(context.Background()); err != nil || *runs != 0 || out.Len() == 0 {
		t.Fatalf("err %v runs %d", err, *runs)
	}
}
