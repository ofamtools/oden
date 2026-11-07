package oden

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
)

// WithBuiltins adds human-facing help, info, version and which, and machine-facing
// commands, catalog, state and command list|show|state. Machine commands
// retain their JSON formats and argument handling. An app's own command
// always owns its name over a builtin. The machine commands yield to
// the app's fallback when WithClaims claims the argv.
func WithBuiltins() Option {
	return func(a *App) {
		cmds := builtins(a)
		for i := range cmds {
			cmds[i].builtin = true
		}
		a.add(1, cmds...)
	}
}

func builtins(a *App) []Command {
	return []Command{
		{Name: "help", Aliases: []string{"-h", "--help"}, Summary: "list the commands by audience", Run: func(ctx context.Context, args []string) error {
			if len(args) != 0 && !(len(args) == 1 && (helpFlag(args[0]) || args[0] == "help")) {
				res := a.Catalog().Resolve(args[0])
				if !res.Resolved() || res.Entry.Name == "help" {
					return &ExitError{Code: 1, Err: fmt.Errorf("%s: unknown or ambiguous command %q", a.helpName(), args[0])}
				}
				return a.RunEntry(ctx, res.Entry, append(args[1:], "--help"))
			}
			a.Help(a.Stdout)
			return nil
		}},
		{Name: "version", Summary: "print the version, commit and build metadata", Run: func(_ context.Context, args []string) error {
			usage := fmt.Sprintf("usage: %s version [-h|--help]", a.Name)
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				_, err := fmt.Fprintln(a.Stdout, usage)
				return err
			}
			if len(args) != 0 {
				return &ExitError{Code: 2, Err: fmt.Errorf("%s", usage)}
			}
			info, _ := debug.ReadBuildInfo()
			return writeVersion(a.Stdout, a.Name, info)
		}},
		{Name: "info", Summary: "describe this bin or a command", Run: func(_ context.Context, args []string) error {
			literal := len(args) > 0 && args[0] == "--"
			if literal {
				args = args[1:]
			}
			usage := fmt.Sprintf("usage: %s info [COMMAND] [-h|--help]", a.Name)
			if !literal && len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				_, err := fmt.Fprintln(a.Stdout, usage)
				return err
			}
			if len(args) > 1 || !literal && len(args) == 1 && strings.HasPrefix(args[0], "-") {
				return &ExitError{Code: 2, Err: fmt.Errorf("%s", usage)}
			}
			if len(args) == 0 {
				if _, err := fmt.Fprintf(a.Stdout, "%s\nVersion: %s\nRoot: %s\n\n", a.Name, buildVersion(), a.RootName()); err != nil {
					return err
				}
				a.Help(a.Stdout)
				return nil
			}
			res := a.Catalog().Resolve(args[0])
			if !res.Resolved() {
				return &ExitError{Code: 1, Err: fmt.Errorf("%s: no command %q (%d match)", a.Name, res.Query, len(res.Candidates))}
			}
			e := res.Entry
			category := e.Category
			if category == "" {
				category = "Other"
			}
			audience := e.Audience
			if audience == "" {
				audience = AudienceHuman
			}
			_, err := fmt.Fprintf(a.Stdout, "%s %s\nSummary: %s\nAudience: %s\nCategory: %s\nKind: %s\nSource: %s\nAliases: %s\n", a.Name, e.Name, e.Summary, audience, category, e.Kind, e.Source, strings.Join(e.Aliases, ", "))
			if err == nil && len(e.Shadowed) != 0 {
				a.List(a.Stdout, []*Entry{e})
			}
			return err
		}},
		{Name: "commands", Audience: AudienceMachine, Yield: true, Summary: "print the command registry as JSON", Run: func(context.Context, []string) error {
			return a.writeJSON(a.Catalog().Entries)
		}},
		{Name: "catalog", Audience: AudienceMachine, Yield: true, Summary: "print the catalog answer (schema 1) as JSON", Run: func(context.Context, []string) error {
			return a.WriteCatalog(a.Stdout)
		}},
		{Name: "state", Audience: AudienceMachine, Yield: true, Summary: "print command rows as JSON Lines", Run: func(context.Context, []string) error {
			return a.WriteState(a.Stdout)
		}},
		{Name: CommandNoun, Audience: AudienceMachine, Yield: true, Summary: "list, show or stream command rows as JSON", Run: func(_ context.Context, args []string) error {
			usage := fmt.Errorf("usage: %s %s list|show NAME|state", a.Name, CommandNoun)
			if len(args) == 0 {
				return usage
			}
			switch args[0] {
			case "list":
				return a.writeList(a.Stdout)
			case "state":
				return a.WriteState(a.Stdout)
			case "show":
				if len(args) != 2 {
					return fmt.Errorf("usage: %s %s show NAME", a.Name, CommandNoun)
				}
				res := a.Catalog().Resolve(args[1])
				if !res.Resolved() {
					return &ExitError{Code: 1, Err: fmt.Errorf("%s: no command %q (%d match)", a.Name, res.Query, len(res.Candidates))}
				}
				return a.writeJSON(commandRow(res.Entry))
			}
			return usage
		}},
		{Name: "which", Summary: "print what a command word resolves to", Run: func(_ context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: %s which COMMAND", a.Name)
			}
			res := a.Catalog().Resolve(args[0])
			switch {
			case res.Ambiguous:
				fmt.Fprintf(a.Stderr, "%s: %q is ambiguous; did you mean one of these?\n", a.Name, res.Query)
				a.List(a.Stderr, res.Candidates)
				return &ExitError{Code: 1}
			case !res.Resolved():
				return &ExitError{Code: 1, Err: fmt.Errorf("%s: no command %q", a.Name, res.Query)}
			}
			e := res.Entry
			_, err := fmt.Fprintf(a.Stdout, "%s %s %s\n", e.Name, e.Kind, e.Source)
			return err
		}},
	}
}

func (a *App) writeJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.Stdout, "%s\n", data)
	return err
}
