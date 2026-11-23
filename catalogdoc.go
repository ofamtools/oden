package oden

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// CatalogSchema is the catalog format version CatalogDoc writes: the
// schema ochazuke/cli.Catalog writes, so a host (nabe, o-inventory) reads
// a plain oden bin and an ochazuke-backed one alike.
const CatalogSchema = 1

// CommandNoun is the one noun a plain oden bin's catalog has: its
// commands, the catalog entries.
const CommandNoun = "command"

// CatalogDoc is a bin's catalog answer in the ochazuke/cli schema. A plain
// oden bin has one noun, command, whose rows are its catalog entries; it
// has no bindings and no pages. State is "command.state".
type CatalogDoc struct {
	Schema   int           `json:"schema"`
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	State    string        `json:"state,omitempty"`
	Nouns    []CatalogNoun `json:"nouns"`
	Bindings []any         `json:"bindings"`
	Pages    []any         `json:"pages"`
}

// CatalogNoun is a noun and its verbs, sorted by name.
type CatalogNoun struct {
	Name  string        `json:"name"`
	Pin   string        `json:"pin,omitempty"`
	Verbs []CatalogVerb `json:"verbs"`
}

// CatalogVerb is one verb, in ochazuke/cli.CatalogVerb's field names.
type CatalogVerb struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Kind        string                  `json:"kind,omitempty"`
	Params      map[string]CatalogParam `json:"params,omitempty"`
	Rows        string                  `json:"rows,omitempty"`
	Quiet       bool                    `json:"quiet,omitempty"`
	Positional  []string                `json:"positional,omitempty"`
}

// CatalogParam is a verb parameter, in ochazuke.Parameter's field names.
type CatalogParam struct {
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

// Row is one line of a state answer, ochazuke/cli.Row's shape:
//
//	{"noun":"command","id":"proj","label":"proj  external  /home/u/bin/o-proj","data":{...}}
type Row struct {
	Noun  string `json:"noun"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// CommandData is a command row's record: how the entry runs, where from,
// its aliases and every claim it shadows.
type CommandData struct {
	Kind       Kind        `json:"kind"`
	Category   string      `json:"category,omitempty"`
	Summary    string      `json:"summary,omitempty"`
	Audience   Audience    `json:"audience,omitempty"`
	Path       string      `json:"path,omitempty"` // an external's executable
	Source     string      `json:"source"`
	Aliases    []string    `json:"aliases"`
	Shadows    []Claim     `json:"shadows"`
	Prefix     string      `json:"prefix,omitempty"` // an external's <prefix>-
	Sub        string      `json:"sub,omitempty"`    // a merged external's subcommand
	Usage      string      `json:"usage,omitempty"`
	Parameters []Parameter `json:"parameters,omitempty"`
}

// CatalogDoc is the bin's catalog answer: the command noun with list
// (rows command), show (positional name) and state (the catalog's state).
// Version is the main module's build version.
func (a *App) CatalogDoc() CatalogDoc {
	return CatalogDoc{
		Schema: CatalogSchema, Name: a.Name, Version: buildVersion(), State: CommandNoun + ".state",
		Nouns: []CatalogNoun{{Name: CommandNoun, Verbs: []CatalogVerb{
			{Name: "list", Description: "every command of " + a.Name + " (inline and external, with what each shadows): {id, label, data}", Kind: "application", Rows: CommandNoun},
			{Name: "show", Description: "one command by name, alias or unique prefix", Kind: "application",
				Params: map[string]CatalogParam{"name": {Type: "string", Required: true, Description: "a command name, alias or unique prefix"}}, Positional: []string{"name"}},
			{Name: "state", Description: "every command row as JSON Lines {noun: command, id, label, data} (the catalog's state)", Kind: "application"},
		}}},
		Bindings: []any{}, Pages: []any{},
	}
}

// WriteCatalog writes CatalogDoc as indented JSON.
func (a *App) WriteCatalog(w io.Writer) error {
	data, err := json.MarshalIndent(a.CatalogDoc(), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// CommandRows are the command rows, one per catalog entry, by name: id is
// the name, label "NAME  KIND  PATH-OR-SOURCE".
func (a *App) CommandRows() []Row {
	entries := a.Catalog().Entries
	out := make([]Row, len(entries))
	for i, e := range entries {
		out[i] = commandRow(e)
	}
	return out
}

func commandRow(e *Entry) Row {
	where := e.Source
	if e.Path != "" {
		where = e.Path
	}
	shadows := e.Shadowed
	if shadows == nil {
		shadows = []Claim{}
	}
	aliases := e.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return Row{Noun: CommandNoun, ID: e.Name, Label: strings.Join([]string{e.Name, string(e.Kind), where}, "  "),
		Data: CommandData{Kind: e.Kind, Category: e.Category, Summary: e.Summary, Audience: e.Audience, Path: e.Path, Source: e.Source, Aliases: aliases, Shadows: shadows, Prefix: e.Prefix, Sub: e.Sub, Usage: e.Usage, Parameters: e.Parameters}}
}

// WriteState writes the command rows as JSON Lines.
func (a *App) WriteState(w io.Writer) error {
	for _, r := range a.CommandRows() {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
			return err
		}
	}
	return nil
}

// writeList writes the command rows as a JSON array, one compact row per
// line (how ochazuke/cli prints a list).
func (a *App) writeList(w io.Writer) error {
	rows := a.CommandRows()
	lines := make([]string, len(rows))
	for i, r := range rows {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		lines[i] = string(data)
	}
	if len(lines) == 0 {
		_, err := fmt.Fprintln(w, "[]")
		return err
	}
	_, err := fmt.Fprintf(w, "[\n  %s\n]\n", strings.Join(lines, ",\n  "))
	return err
}

// buildVersion is the main module's build version, "" when unknown.
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}
