package oden

import (
	"sort"
	"strings"
)

// Kind says how an entry runs: an inline Command in this process, or an
// external executable found on PATH.
type Kind string

const (
	KindInline   Kind = "inline"
	KindExternal Kind = "external"
)

// External is an executable named <Prefix>-<Name> found on PATH. Prefix
// is the bin itself or one of its extra prefixes (WithPrefixes).
type External struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Prefix string `json:"prefix,omitempty"`
}

// Claim is a losing claim on a token: a later inline registration, an
// external that lost to an inline command or to an earlier PATH entry, or
// an alias that names a command already owned.
type Claim struct {
	Name    string `json:"name"`
	Kind    Kind   `json:"kind"`
	Source  string `json:"source"`
	AliasOf string `json:"alias_of,omitempty"`
	// Merged marks the external a command group merges: its subcommands
	// join the group's children (see App.RunEntry).
	Merged bool `json:"merged,omitempty"`

	builtin bool // the losing claim is an opt-in builtin (WithBuiltins)
}

// Audience identifies a command's intended consumer, independently of its
// domain category. Unspecified commands are presented as human-facing.
type Audience string

const (
	AudienceHuman   Audience = "human"
	AudienceMachine Audience = "machine"
)

// Entry is one command of a catalog: the winning claim on its name, its
// aliases and every claim it shadows.
type Entry struct {
	Name     string   `json:"name"`
	Kind     Kind     `json:"kind"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary,omitempty"`
	Category string   `json:"category,omitempty"`
	Audience Audience `json:"audience,omitempty"`
	Source   string   `json:"source"`
	Prefix   string   `json:"prefix,omitempty"` // an external's <prefix>-
	Shadowed []Claim  `json:"shadowed"`

	// Sub is set for a subcommand of a merged external: it runs as
	// Path Sub ARGS... (BIN-NAME SUB ARGS...).
	Sub string `json:"sub,omitempty"`

	Command *Command `json:"-"` // set for KindInline
	Path    string   `json:"-"` // set for KindExternal

	parent string // a merged subcommand's ODEN_PARENT
}

// Resolution is what a word names: the entry when it names exactly one
// (Exact for a name or alias, else a unique prefix), or the candidates
// when it is a prefix of several.
type Resolution struct {
	Query      string
	Entry      *Entry
	Exact      bool
	Ambiguous  bool
	Candidates []*Entry
}

// Resolved reports whether the word names one entry.
func (r Resolution) Resolved() bool { return r.Entry != nil }

// Catalog is the commands one bin can run: its inline commands merged with
// the externals discovered on PATH. Ownership of a token, highest first:
//
//  1. an inline command's name (the first registration wins)
//  2. an external's name (the first on PATH wins)
//  3. an inline alias (the first registration wins)
//
// Every losing claim is kept in the winner's Shadowed list.
type Catalog struct {
	Bin     string
	Entries []*Entry // sorted by name
	tokens  map[string]*Entry
}

// NewCatalog merges inline commands (in registration order), externals (in
// PATH order) and the externals that lost to an earlier PATH entry.
func NewCatalog(bin string, inline []*Command, externals, shadowedExternals []External) *Catalog {
	c := &Catalog{Bin: bin, tokens: map[string]*Entry{}}
	byName := map[string]*Entry{}
	for _, cmd := range inline {
		if w := byName[cmd.Name]; w != nil {
			w.Shadowed = append(w.Shadowed, Claim{Name: cmd.Name, Kind: KindInline, Source: cmd.Source, builtin: cmd.builtin})
			if cmd.builtin && w.Audience == "" {
				w.Audience = cmd.Audience
			}
			continue
		}
		summary := cmd.Summary
		if summary == "" && cmd.Children != nil {
			names := make([]string, 0, len(cmd.Children))
			for _, child := range cmd.Children {
				names = append(names, child.Name)
			}
			sort.Strings(names)
			summary = strings.Join(names, " ")
		}
		byName[cmd.Name] = &Entry{Name: cmd.Name, Kind: KindInline, Aliases: []string{}, Summary: summary, Category: cmd.Category, Audience: cmd.Audience, Source: cmd.Source, Shadowed: []Claim{}, Command: cmd}
	}
	for _, ext := range externals {
		if w := byName[ext.Name]; w != nil {
			merged := w.Kind == KindInline && w.Command.Children != nil
			w.Shadowed = append(w.Shadowed, Claim{Name: ext.Name, Kind: KindExternal, Source: ext.Path, Merged: merged})
			continue
		}
		byName[ext.Name] = &Entry{Name: ext.Name, Kind: KindExternal, Aliases: []string{}, Source: ext.Path, Prefix: ext.Prefix, Shadowed: []Claim{}, Path: ext.Path}
	}
	for _, ext := range shadowedExternals {
		if w := byName[ext.Name]; w != nil {
			w.Shadowed = append(w.Shadowed, Claim{Name: ext.Name, Kind: KindExternal, Source: ext.Path})
		}
	}
	for _, e := range byName {
		c.Entries = append(c.Entries, e)
		c.tokens[e.Name] = e
	}
	sort.Slice(c.Entries, func(i, j int) bool { return c.Entries[i].Name < c.Entries[j].Name })
	for _, cmd := range inline {
		owner := byName[cmd.Name]
		if owner.Command != cmd {
			continue
		}
		for _, alias := range cmd.Aliases {
			if w := c.tokens[alias]; w != nil {
				w.Shadowed = append(w.Shadowed, Claim{Name: alias, Kind: KindInline, Source: cmd.Source, AliasOf: cmd.Name})
				continue
			}
			owner.Aliases = append(owner.Aliases, alias)
			c.tokens[alias] = owner
		}
	}
	return c
}

// Resolve finds what word names: an exact name or alias, else the one
// entry with a token starting with word. Descendants never make a prefix
// ambiguous: a match named <other>-... where <other> also matches is
// dropped, so bio resolves to biographies over biographies-freddie-kruger,
// while biographies and books stay ambiguous. These are hiiro's semantics
// (~/proj/hiiro/lib/hiiro/catalog.rb:56-57).
func (c *Catalog) Resolve(word string) Resolution {
	res := Resolution{Query: word}
	if word == "" {
		return res
	}
	if e := c.tokens[word]; e != nil {
		res.Entry, res.Exact, res.Candidates = e, true, []*Entry{e}
		return res
	}
	seen := map[*Entry]bool{}
	var matches []*Entry
	for token, e := range c.tokens {
		if strings.HasPrefix(token, word) && !seen[e] {
			seen[e] = true
			matches = append(matches, e)
		}
	}
	var kept []*Entry
	for _, e := range matches {
		nested := false
		for _, o := range matches {
			if o != e && strings.HasPrefix(e.Name, o.Name+"-") {
				nested = true
				break
			}
		}
		if !nested {
			kept = append(kept, e)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })
	res.Candidates = kept
	switch len(kept) {
	case 0:
	case 1:
		res.Entry = kept[0]
	default:
		res.Ambiguous = true
	}
	return res
}

// Lookup is the entry owning an exact name or alias, or nil.
func (c *Catalog) Lookup(token string) *Entry { return c.tokens[token] }
