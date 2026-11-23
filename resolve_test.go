package oden

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestResolveCandidatePriority(t *testing.T) {
	items := []Candidate{
		{ID: "ab12", Title: "Alpha Build"},
		{ID: "AB12", Title: "Upper Build"},
		{ID: "cd34", Title: "Beta Deploy"},
		{ID: "cd56", Title: "Gamma Deploy"},
		{ID: "ef78", Title: "Unique Title"},
	}

	// 1a. Exact case-sensitive ID match wins over case-insensitive match.
	c, err := ResolveCandidate(context.Background(), "ab12", items, ResolveOptions{Noun: "task"})
	if err != nil || c.ID != "ab12" {
		t.Fatalf("exact case-sensitive ID: got %+v, %v", c, err)
	}

	// 1b. Exact case-insensitive ID match.
	c, err = ResolveCandidate(context.Background(), "EF78", items, ResolveOptions{Noun: "task"})
	if err != nil || c.ID != "ef78" {
		t.Fatalf("exact case-insensitive ID: got %+v, %v", c, err)
	}

	// 2. Unique case-insensitive ID prefix match.
	c, err = ResolveCandidate(context.Background(), "CD3", items, ResolveOptions{Noun: "task"})
	if err != nil || c.ID != "cd34" {
		t.Fatalf("unique ID prefix: got %+v, %v", c, err)
	}

	// 3. Exact case-insensitive Title match.
	c, err = ResolveCandidate(context.Background(), "unique title", items, ResolveOptions{Noun: "task"})
	if err != nil || c.ID != "ef78" {
		t.Fatalf("exact title: got %+v, %v", c, err)
	}

	// 4. Unique case-insensitive Title substring match.
	c, err = ResolveCandidate(context.Background(), "gamma", items, ResolveOptions{Noun: "task"})
	if err != nil || c.ID != "cd56" {
		t.Fatalf("title substring: got %+v, %v", c, err)
	}

	// 7. Zero matches -> clear not found error.
	_, err = ResolveCandidate(context.Background(), "zzz", items, ResolveOptions{Noun: "task"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveCandidateAmbiguousAndEmpty(t *testing.T) {
	items := []Candidate{
		{ID: "cd34", Title: "Beta Deploy"},
		{ID: "cd56", Title: "Gamma Deploy"},
		{ID: "ef78", Title: "Other"},
	}

	// Non-interactive ambiguous -> *AmbiguousError listing each candidate.
	_, err := ResolveCandidate(context.Background(), "deploy", items, ResolveOptions{Noun: "lane"})
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("expected *AmbiguousError, got %T (%v)", err, err)
	}
	if len(amb.Candidates) != 2 {
		t.Fatalf("expected 2 ambiguous candidates, got %d", len(amb.Candidates))
	}
	msg := amb.Error()
	if !strings.Contains(msg, "cd34  Beta Deploy") || !strings.Contains(msg, "cd56  Gamma Deploy") {
		t.Fatalf("ambiguous error missing candidate lines:\n%s", msg)
	}

	// Interactive ambiguous -> Picker receives only the ambiguous candidates.
	var pickedFrom []Candidate
	picked, err := ResolveCandidate(context.Background(), "deploy", items, ResolveOptions{
		Noun:        "lane",
		Interactive: true,
		Pick: func(_ context.Context, title, query string, cands []Candidate) (Candidate, bool, error) {
			pickedFrom = cands
			return cands[1], true, nil
		},
	})
	if err != nil || picked.ID != "cd56" {
		t.Fatalf("interactive ambiguous: got %+v, %v", picked, err)
	}
	if len(pickedFrom) != 2 {
		t.Fatalf("picker should receive only ambiguous candidates (2), got %d", len(pickedFrom))
	}

	// Non-interactive empty query -> usage error.
	_, err = ResolveCandidate(context.Background(), "", items, ResolveOptions{Noun: "lane"})
	if err == nil || !strings.Contains(err.Error(), "lane id or title is required") {
		t.Fatalf("empty query non-interactive: got %v", err)
	}

	// Interactive empty query -> Picker receives all items.
	picked, err = ResolveCandidate(context.Background(), "", items, ResolveOptions{
		Noun:        "lane",
		Interactive: true,
		Pick: func(_ context.Context, title, query string, cands []Candidate) (Candidate, bool, error) {
			pickedFrom = cands
			return cands[0], true, nil
		},
	})
	if err != nil || picked.ID != "cd34" || len(pickedFrom) != 3 {
		t.Fatalf("interactive empty: got %+v, len=%d, err=%v", picked, len(pickedFrom), err)
	}
}

func TestWithPickerInheritanceAndCtxResolve(t *testing.T) {
	origCheck := TerminalCheck
	TerminalCheck = func(io.Reader, io.Writer) bool { return true }
	t.Cleanup(func() { TerminalCheck = origCheck })

	var out bytes.Buffer
	app := New("demo", []string{"lane", "show"},
		WithPath(t.TempDir()),
		WithStdio(strings.NewReader(""), &out, &out),
		WithPicker(func(_ context.Context, title, query string, items []Candidate) (Candidate, bool, error) {
			return items[len(items)-1], true, nil
		}),
	)
	app.Add(Command{
		Name: "lane",
		Children: []Command{
			{
				Name: "show",
				Handle: func(c *Ctx) error {
					cand, err := c.Resolve(c.Arg(0, ""), []Candidate{
						{ID: "l1", Title: "One"},
						{ID: "l2", Title: "Two"},
					}, "lane")
					if err != nil {
						return err
					}
					return c.Println(cand.ID)
				},
			},
		},
	})
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.TrimSpace(out.String()) != "l2" {
		t.Fatalf("expected picker result l2, got %q", out.String())
	}
}
