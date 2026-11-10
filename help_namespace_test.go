package oden

import (
	"context"
	"strings"
	"testing"
)

func namespaceApp(t *testing.T) (*App, func(args ...string) string) {
	t.Helper()
	app, out, _ := testApp(t, fakeFS{}, "", WithBuiltins())
	app.HelpText = "FULL NOTES\nsecond\nthird\nfourth"
	app.HelpBrief = "BRIEF NOTES"
	app.Add(
		Command{Name: "own", Summary: "the bin's own", Run: noop},
		Command{Name: "completion", Summary: "script", Run: noop},
		Command{Name: "fw.", Aliases: []string{"fw"}, Summary: "(+2 nouns)", Namespace: "fw", NamespaceHead: true, Children: []Command{{Name: "x", Run: noop}}},
		Command{Name: "fw.alpha", Aliases: []string{"alpha", "completion"}, Summary: "alpha noun", Namespace: "fw", Run: noop},
		Command{Name: "fw.__state", Summary: "internal", Namespace: "fw", Hidden: true, Run: noop},
		Command{Name: "secret", Summary: "internal own", Hidden: true, Run: noop},
	)
	return app, func(args ...string) string {
		out.Reset()
		app.Args = args
		if err := app.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
}

func TestDefaultHelpListsOwnCommandsAndOneLinePerNamespace(t *testing.T) {
	_, run := namespaceApp(t)
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		got := run(args...)
		for _, want := range []string{"own", "completion", "help", "Namespaces:", "fw., fw", "(+2 nouns): mtm help --all, or mtm fw", "BRIEF NOTES"} {
			if !strings.Contains(got, want) {
				t.Fatalf("%v: missing %q in\n%s", args, want, got)
			}
		}
		for _, bad := range []string{"fw.alpha", "alpha noun", "__state", "secret", "shadows", "FULL NOTES"} {
			if strings.Contains(got, bad) {
				t.Fatalf("%v: default help has %q:\n%s", args, bad, got)
			}
		}
	}
}

func TestHelpAllListsEverything(t *testing.T) {
	_, run := namespaceApp(t)
	for _, args := range [][]string{{"help", "--all"}, {"--help", "--all"}, {"-h", "--all"}} {
		got := run(args...)
		for _, want := range []string{"fw.alpha", "fw.__state", "secret", "shadows alias completion of fw.alpha", "FULL NOTES", "fw., fw"} {
			if !strings.Contains(got, want) {
				t.Fatalf("%v: missing %q in\n%s", args, want, got)
			}
		}
		if strings.Contains(got, "Namespaces:") || strings.Contains(got, "BRIEF NOTES") {
			t.Fatalf("%v: --all is the flat listing:\n%s", args, got)
		}
	}
}

func TestNamespaceWithoutHeadGetsACountLine(t *testing.T) {
	app, out, _ := testApp(t, fakeFS{}, "")
	app.Add(
		Command{Name: "a", Namespace: "ns", Run: noop},
		Command{Name: "b", Namespace: "ns", Run: noop},
		Command{Name: "own", Run: noop},
	)
	app.Help(out)
	if got := out.String(); !strings.Contains(got, "ns  (+2 commands): mtm help --all, or mtm ns") || strings.Contains(got, "  a ") {
		t.Fatalf("help:\n%s", got)
	}
}

func TestHiddenAndNamespaceRowsStayInMachineOutput(t *testing.T) {
	_, run := namespaceApp(t)
	got := run("commands")
	for _, want := range []string{"fw.alpha", "fw.__state", "secret"} {
		if !strings.Contains(got, want) {
			t.Fatalf("commands lacks %q:\n%s", want, got)
		}
	}
}

func TestReplacedBuiltinClaimIsOnlyInHelpAll(t *testing.T) {
	app, out, _ := testApp(t, fakeFS{}, "", WithBuiltins())
	app.Add(Command{Name: "catalog", Summary: "mine", Run: noop})
	app.Help(out)
	if strings.Contains(out.String(), "shadows") {
		t.Fatalf("default help:\n%s", out)
	}
	out.Reset()
	app.HelpAll(out)
	if !strings.Contains(out.String(), "shadows inline") {
		t.Fatalf("help --all:\n%s", out)
	}
}
