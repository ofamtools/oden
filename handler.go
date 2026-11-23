package oden

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
)

// Handler runs an inline command with its arguments and streams.
type Handler func(*Ctx) error

// Ctx is one invocation. Name is the canonical command name, not its alias.
// Input helpers open OS files; WithFS configures discovery only.
type Ctx struct {
	context.Context
	App            *App
	Name           string
	Args           []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Picker         PickerFunc
}

// Cmd registers a handler and records the caller as its source.
func (a *App) Cmd(name, summary string, run Handler, aliases ...string) *App {
	return a.add(2, Command{Name: name, Summary: summary, Handle: run, Aliases: aliases})
}

// Arg returns a positional argument or def when i is out of bounds.
func (c *Ctx) Arg(i int, def string) string {
	if i < 0 || i >= len(c.Args) {
		return def
	}
	return c.Args[i]
}

// Open reads paths in order, opening one file at a time. "-" means stdin;
// no paths means stdin. Open and read errors are returned by Read.
// Close releases the current file but never closes stdin. Callers must close it.
func (c *Ctx) Open(paths ...string) io.ReadCloser {
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	return &inputReader{paths: append([]string(nil), paths...), stdin: c.Stdin}
}

// Input treats Args as file paths, like cat. Parse flags before calling Open
// explicitly if the command accepts anything other than paths.
func (c *Ctx) Input() io.ReadCloser { return c.Open(c.Args...) }

// Text reads all input and closes any opened file.
func (c *Ctx) Text() (string, error) {
	r := c.Input()
	defer r.Close()
	b, err := io.ReadAll(r)
	return string(b), err
}

// Lines yields lines without newlines, then any read error once. It closes
// files even when iteration stops early. Scanner's default 64 KiB limit applies.
func (c *Ctx) Lines() iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		r := c.Input()
		defer r.Close()
		s := bufio.NewScanner(r)
		for s.Scan() {
			if !yield(s.Text(), nil) {
				return
			}
		}
		if err := s.Err(); err != nil {
			yield("", err)
		}
	}
}

// Printf writes formatted output to the command's stdout.
func (c *Ctx) Printf(f string, a ...any) error { _, err := fmt.Fprintf(c.Stdout, f, a...); return err }

// Println writes a line to the command's stdout.
func (c *Ctx) Println(a ...any) error { _, err := fmt.Fprintln(c.Stdout, a...); return err }

// Usage prints usage to stderr and returns exit code 2.
func (c *Ctx) Usage(synopsis string) error {
	_, err := fmt.Fprintf(c.Stderr, "usage: %s %s", c.App.Name, c.Name)
	if err == nil && synopsis != "" {
		_, err = fmt.Fprintf(c.Stderr, " %s", synopsis)
	}
	if err == nil {
		_, err = fmt.Fprintln(c.Stderr)
	}
	return &ExitError{Code: 2, Err: err}
}

// Sub creates a child app over the remaining arguments and inherited streams.
// Options override inherited settings; extra prefixes are not inherited.
func (c *Ctx) Sub(opts ...Option) *App {
	a := Child(c.App, c.Name, c.Args)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type inputReader struct {
	paths   []string
	stdin   io.Reader
	current io.Reader
	file    *os.File
	closed  bool
	err     error
}

func (r *inputReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, os.ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if r.err != nil {
			return 0, r.err
		}
		if r.current == nil {
			if len(r.paths) == 0 {
				return 0, io.EOF
			}
			path := r.paths[0]
			r.paths = r.paths[1:]
			if path == "-" {
				r.current = r.stdin
			} else {
				r.file, r.err = os.Open(path)
				if r.err != nil {
					return 0, r.err
				}
				r.current = r.file
			}
		}
		n, err := r.current.Read(p)
		if err == io.EOF {
			if r.file != nil {
				r.err = r.file.Close()
				r.file = nil
			}
			r.current = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		if err != nil {
			r.err = err
		}
		return n, err
	}
}

func (r *inputReader) Close() error {
	r.closed = true
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}
