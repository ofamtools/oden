package oden

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

// Parameter describes a command argument for help. Oden does not parse it.
type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Choices     []string
	Source      string
	Free        bool
}

func helpFlag(word string) bool { return word == "-h" || word == "--help" }

func (a *App) helpName() string {
	if a.invocation != "" {
		return a.invocation
	}
	return a.Name
}

func (a *App) helpNotes(w io.Writer) {
	if a.HelpText != "" {
		fmt.Fprintf(w, "\n%s\n", a.HelpText)
	}
}

// helpNotesBrief writes HelpBrief when set, else the full notes.
func (a *App) helpNotesBrief(w io.Writer) {
	if a.HelpBrief != "" {
		fmt.Fprintf(w, "\n%s\n", a.HelpBrief)
		return
	}
	a.helpNotes(w)
}

func (a *App) commandHelp(w io.Writer, c *Command) error {
	var out strings.Builder
	usage := c.Usage
	if usage == "" {
		usage = "[ARGS...]"
	}
	fmt.Fprintf(&out, "usage: %s %s %s\n", a.helpName(), c.Name, usage)
	if c.Summary != "" {
		fmt.Fprintf(&out, "\n%s\n", c.Summary)
	}
	if c.Details != "" {
		fmt.Fprintf(&out, "\n%s\n", c.Details)
	}
	writeParameters(&out, c.Parameters, "  ")
	a.helpNotes(&out)
	_, err := io.WriteString(w, out.String())
	return err
}
func writeParameters(w io.Writer, parameters []Parameter, indent string) {
	for _, p := range parameters {
		required := "optional"
		if p.Required {
			required = "required"
		}
		fmt.Fprintf(w, "\n%s--%s <%s> (%s)", indent, p.Name, p.Type, required)
		if p.Description != "" {
			fmt.Fprintf(w, ": %s", p.Description)
		}
		fmt.Fprintln(w)
		if len(p.Choices) != 0 {
			fmt.Fprintf(w, "%s  choices: %s\n", indent, strings.Join(p.Choices, ", "))
		}
		if p.Source != "" {
			fmt.Fprintf(w, "%s  choices from: %s\n", indent, p.Source)
		}
		if p.Free {
			fmt.Fprintf(w, "%s  other values are accepted\n", indent)
		}
	}
}

// Help writes the default help: the bin's own commands separated by
// audience, then by optional category, and one line per namespace (a
// Command's Namespace) instead of its members. Hidden commands and the
// shadowed-claim lines are left out; HelpAll lists everything.
func (a *App) Help(w io.Writer) { a.help(w, false) }

// HelpAll writes the full help: every command, hidden ones and namespace
// members included, with the claims each one shadows.
func (a *App) HelpAll(w io.Writer) { a.help(w, true) }

// namespaceOf is the namespace an entry's inline command belongs to.
func namespaceOf(e *Entry) string {
	if e.Command == nil {
		return ""
	}
	return e.Command.Namespace
}

func (a *App) help(w io.Writer, all bool) {
	defer func() {
		if a.merge != nil && a.merge.passThrough {
			fmt.Fprintf(w, "\nOther words run %s (%s).\n", a.merge.name, a.merge.path)
		}
		hasGroups := a.invocation != ""
		for _, e := range a.Catalog().Entries {
			hasGroups = hasGroups || e.Command != nil && e.Command.Children != nil
		}
		if hasGroups {
			fmt.Fprintf(w, "\nRun '%s COMMAND --help' for command details.\n", a.helpName())
		}
		if all {
			a.helpNotes(w)
			return
		}
		a.helpNotesBrief(w)
	}()
	fmt.Fprintf(w, "usage: %s COMMAND [ARGS...]\n\n", a.helpName())
	entries := a.Catalog().Entries
	var human, machine, heads []*Entry
	members := map[string]int{}
	var order []string
	for _, e := range entries {
		ns := namespaceOf(e)
		if !all {
			if ns != "" {
				if !slices.Contains(order, ns) {
					order = append(order, ns)
				}
				switch {
				case e.Command.NamespaceHead:
					heads = append(heads, e)
				case !e.Command.Hidden:
					members[ns]++
				}
				continue
			}
			if e.Command != nil && e.Command.Hidden {
				continue
			}
		}
		if e.Audience == AudienceMachine {
			machine = append(machine, e)
		} else {
			human = append(human, e)
		}
	}
	list := func(entries []*Entry) { a.list(w, entries, all) }
	lines := a.namespaceLines(order, heads, members)
	if len(machine) == 0 {
		fmt.Fprintln(w, "commands:")
		list(human)
		a.writeNamespaces(w, lines)
		return
	}
	if len(human) != 0 {
		fmt.Fprintln(w, "Human commands:")
		list(human)
		a.writeNamespaces(w, lines)
		fmt.Fprintln(w)
	} else {
		a.writeNamespaces(w, lines)
	}
	fmt.Fprintln(w, "Machine commands (JSON / JSON Lines):")
	list(machine)
}

type namespaceLine struct{ names, summary string }

// namespaceLines builds one line per namespace, in first-seen order: the
// namespace's head command when it has one (its names and summary), else
// the namespace's name and its member count. Each line says how to list
// the members.
func (a *App) namespaceLines(order []string, heads []*Entry, members map[string]int) []namespaceLine {
	var lines []namespaceLine
	for _, ns := range order {
		line := namespaceLine{names: ns, summary: fmt.Sprintf("(+%d commands)", members[ns])}
		word := ns
		for _, h := range heads {
			if h.Command.Namespace == ns {
				line.names = strings.Join(append([]string{h.Name}, h.Aliases...), ", ")
				if h.Summary != "" {
					line.summary = h.Summary
				}
				word = h.Name
				if len(h.Aliases) != 0 {
					word = h.Aliases[0]
				}
				break
			}
		}
		line.summary += fmt.Sprintf(": %s help --all, or %s %s", a.helpName(), a.helpName(), word)
		lines = append(lines, line)
	}
	return lines
}

func (a *App) writeNamespaces(w io.Writer, lines []namespaceLine) {
	if len(lines) == 0 {
		return
	}
	width := 0
	for _, l := range lines {
		width = max(width, len(l.names))
	}
	fmt.Fprintln(w, "\nNamespaces:")
	for _, l := range lines {
		fmt.Fprintf(w, "  %-*s  %s\n", width, l.names, l.summary)
	}
}

// List writes entries grouped by optional category, with uncategorized
// entries under Other. Without categories it keeps a flat listing.
func (a *App) List(w io.Writer, entries []*Entry) { a.list(w, entries, true) }

// list is List; claims says whether claims on a namespace member's name get
// their lines (the others always do).
func (a *App) list(w io.Writer, entries []*Entry, claims bool) {
	categorized := false
	for _, e := range entries {
		if e.Category != "" {
			categorized = true
			break
		}
	}
	if !categorized {
		a.listEntries(w, entries, claims)
		return
	}
	groups := map[string][]*Entry{}
	for _, e := range entries {
		category := e.Category
		if category == "" {
			category = "Other"
		}
		groups[category] = append(groups[category], e)
	}
	keys := make([]string, 0, len(groups))
	for category := range groups {
		keys = append(keys, category)
	}
	sort.Strings(keys)
	for _, category := range keys {
		fmt.Fprintf(w, "\n%s:\n", category)
		a.listEntries(w, groups[category], claims)
	}
}

func (a *App) listEntries(w io.Writer, entries []*Entry, claims bool) {
	names := make([]string, len(entries))
	nameW, sumW := 0, 0
	for i, e := range entries {
		names[i] = strings.Join(append([]string{e.Name}, e.Aliases...), ", ")
		nameW = max(nameW, len(names[i]))
		sumW = max(sumW, len(e.Summary))
	}
	for i, e := range entries {
		line := fmt.Sprintf("  %-*s  %-10s  %-*s  %s", nameW, names[i], "("+string(e.Kind)+")", sumW, e.Summary, e.Source)
		fmt.Fprintln(w, strings.TrimRight(line, " "))
		for _, c := range e.Shadowed {
			if !claims && a.claimIsNamespaced(c) {
				continue // a namespace member's claim is for HelpAll, which, info
			}
			label := string(c.Kind)
			if c.AliasOf != "" {
				label = fmt.Sprintf("alias %s of %s", c.Name, c.AliasOf)
			}
			verb := "shadows"
			if c.Merged {
				verb = "merges"
			}
			fmt.Fprintf(w, "  %-*s  %s %s %s\n", nameW, "", verb, label, c.Source)
		}
		if a.invocation != "" && e.Command != nil {
			for line := range strings.Lines(e.Command.Details) {
				fmt.Fprintf(w, "    %s\n", strings.TrimSuffix(line, "\n"))
			}
			writeParameters(w, e.Command.Parameters, "    ")
		}
	}
}

// claimIsNamespaced reports whether a shadowed alias claim is on a
// command of a namespace or a hidden one, or a builtin that an app command
// replaced.
func (a *App) claimIsNamespaced(c Claim) bool {
	if c.builtin {
		return true
	}
	if c.AliasOf == "" {
		return false
	}
	e := a.Catalog().Lookup(c.AliasOf)
	return e != nil && e.Command != nil && (e.Command.Namespace != "" || e.Command.Hidden)
}
