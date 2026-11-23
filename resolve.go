package oden

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// Candidate is one resolvable target item (e.g. a task, project, lane, or doc).
type Candidate struct {
	ID       string
	Title    string
	Subtitle string
	Badges   []string
	Data     any
}

// PickerFunc interactively picks one Candidate from items. It returns
// (picked, true, nil) on selection, (zero, false, nil) on cancellation, or an error.
type PickerFunc func(ctx context.Context, title, query string, items []Candidate) (Candidate, bool, error)

// ResolveOptions configures ResolveCandidate.
type ResolveOptions struct {
	Noun        string
	Interactive bool
	Pick        PickerFunc
}

// AmbiguousError is returned when a query matches multiple candidates and no
// interactive picker is available. Its Error() lists every candidate so
// non-TTY callers and agents can immediately choose one.
type AmbiguousError struct {
	Noun       string
	Query      string
	Candidates []Candidate
}

func (e *AmbiguousError) Error() string {
	noun := e.Noun
	if noun == "" {
		noun = "item"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous %s %q matches %d candidates:", noun, e.Query, len(e.Candidates))
	for _, c := range e.Candidates {
		if c.Title != "" {
			fmt.Fprintf(&b, "\n  %s  %s", c.ID, c.Title)
		} else {
			fmt.Fprintf(&b, "\n  %s", c.ID)
		}
	}
	return b.String()
}

// TerminalCheck is the predicate used by IsTerminal. Tests may replace it.
var TerminalCheck = defaultIsTerminal

func defaultIsTerminal(r io.Reader, w io.Writer) bool {
	return isCharDevice(r) && isCharDevice(w)
}

func isCharDevice(v any) bool {
	f, ok := v.(*os.File)
	if !ok || f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// IsTerminal reports whether both r and w refer to interactive terminal devices.
func IsTerminal(r io.Reader, w io.Writer) bool {
	if TerminalCheck == nil {
		return defaultIsTerminal(r, w)
	}
	return TerminalCheck(r, w)
}

// ResolveCandidate resolves query against items in priority order:
//  1. Exact ID match (case-sensitive, then case-insensitive)
//  2. Unique case-insensitive ID prefix match
//  3. Exact case-insensitive Title match
//  4. Unique case-insensitive Title substring match
//
// Ambiguous matches invoke opts.Pick when opts.Interactive && opts.Pick != nil,
// or return *AmbiguousError. An empty query invokes opts.Pick over all items
// when interactive, or returns a clear usage error.
func ResolveCandidate(ctx context.Context, query string, items []Candidate, opts ResolveOptions) (Candidate, error) {
	noun := opts.Noun
	if noun == "" {
		noun = "item"
	}
	q := strings.TrimSpace(query)

	pickFrom := func(pickQuery string, list []Candidate, ambiguous bool) (Candidate, error) {
		if opts.Interactive && opts.Pick != nil {
			picked, ok, err := opts.Pick(ctx, "Select "+noun, pickQuery, list)
			if err != nil {
				return Candidate{}, err
			}
			if !ok {
				return Candidate{}, fmt.Errorf("%s selection cancelled", noun)
			}
			return picked, nil
		}
		if ambiguous {
			return Candidate{}, &AmbiguousError{Noun: noun, Query: q, Candidates: list}
		}
		return Candidate{}, fmt.Errorf("%s id or title is required", noun)
	}

	if q == "" {
		if opts.Interactive && opts.Pick != nil {
			if len(items) == 0 {
				return Candidate{}, fmt.Errorf("no %s found", noun)
			}
			return pickFrom("", items, false)
		}
		return Candidate{}, fmt.Errorf("%s id or title is required", noun)
	}

	// 1a. Exact case-sensitive ID match.
	if m := filterCandidates(items, func(c Candidate) bool { return c.ID == q }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return pickFrom(q, m, true)
	}

	// 1b. Exact case-insensitive ID match.
	if m := filterCandidates(items, func(c Candidate) bool { return strings.EqualFold(c.ID, q) }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return pickFrom(q, m, true)
	}

	ql := strings.ToLower(q)

	// 2. Case-insensitive ID prefix match.
	if m := filterCandidates(items, func(c Candidate) bool { return strings.HasPrefix(strings.ToLower(c.ID), ql) }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return pickFrom(q, m, true)
	}

	// 3. Exact case-insensitive Title match.
	if m := filterCandidates(items, func(c Candidate) bool { return strings.EqualFold(c.Title, q) }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return pickFrom(q, m, true)
	}

	// 4. Case-insensitive Title substring match.
	if m := filterCandidates(items, func(c Candidate) bool { return strings.Contains(strings.ToLower(c.Title), ql) }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return pickFrom(q, m, true)
	}

	return Candidate{}, fmt.Errorf("%s %q not found", noun, q)
}

func filterCandidates(items []Candidate, match func(Candidate) bool) []Candidate {
	var out []Candidate
	for _, item := range items {
		if match(item) {
			out = append(out, item)
		}
	}
	return out
}

// Resolve resolves query against items using the command's TTY state and Picker.
func (c *Ctx) Resolve(query string, items []Candidate, noun string) (Candidate, error) {
	return ResolveCandidate(c.Context, query, items, ResolveOptions{
		Noun:        noun,
		Interactive: IsTerminal(c.Stdin, c.Stdout),
		Pick:        c.Picker,
	})
}
