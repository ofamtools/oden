package oden

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	in := map[string]any{"name": "demo", "count": 2, "expr": "a < b"}
	if err := WriteJSON(&buf, in); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "\n  \"name\": \"demo\"") {
		t.Fatalf("expected 2-space indented JSON, got:\n%s", s)
	}
	if !strings.Contains(s, "a < b") {
		t.Fatalf("expected unescaped HTML characters, got:\n%s", s)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	err := WriteTable(&buf,
		[]string{"id", "title", "notes"},
		[][]string{
			{"1", "First item", "line1\nline2\r\nline3"},
			{"22", "Second", "tab\there"},
		},
	)
	if err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "ID") || !strings.Contains(lines[0], "TITLE") || !strings.Contains(lines[0], "NOTES") {
		t.Fatalf("expected uppercase headers, got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "line1 line2 line3") {
		t.Fatalf("expected sanitized newlines, got: %q", lines[1])
	}
	if !strings.Contains(lines[2], "tab here") {
		t.Fatalf("expected sanitized tabs, got: %q", lines[2])
	}
}

func TestCtxWriteHelpers(t *testing.T) {
	var out bytes.Buffer
	app := New("demo", []string{"emit"}, WithPath(t.TempDir()), WithStdio(strings.NewReader(""), &out, &out))
	app.Cmd("emit", "emit output", func(c *Ctx) error {
		if err := c.WriteTable([]string{"k", "v"}, [][]string{{"a", "1"}}); err != nil {
			return err
		}
		return c.WriteJSON(map[string]string{"ok": "yes"})
	})
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "K") || !strings.Contains(got, "\"ok\": \"yes\"") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}
