// Package oden is a small framework for extensible subcommand-driven CLI
// tools: a bin's inline commands merged with the executables named
// <bin>-<sub> on PATH (git- and hiiro-style third-party commands), resolved
// by exact name, alias or unique prefix. It knows no application.
package oden

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Environment variables an external command is started with.
const (
	EnvParent = "ODEN_PARENT" // the bin that ran it
	EnvRoot   = "ODEN_ROOT"   // the outermost bin of the chain
)

// Command is an inline command. Run receives the words after the command's
// own. Source is where it was registered (file:line), filled by Add.
type Command struct {
	Name     string
	Aliases  []string
	Summary  string
	Category string   // optional listing group
	Audience Audience // empty defaults to human; overrides inherit a builtin's audience
	// Yield makes the command give way, when its name is typed exactly, to
	// the app's fallback if App.Claims claims the whole argv. The machine
	// builtins set it; see WithClaims.
	Yield bool
	// Children makes this a command group. A non-nil empty slice is an
	// empty group; nil leaves Run/Handle in control of the arguments.
	Children   []Command
	Usage      string // optional leaf argument synopsis, for contextual help
	Details    string // additional descriptive help text
	Parameters []Parameter
	Run        func(ctx context.Context, args []string) error
	Handle     Handler // takes precedence over Run when set
	Source     string

	builtin bool // WithBuiltins': loses its name to any other inline command
}

// App is one bin: its name, its arguments (without the bin name), its
// inline commands and where it looks for external ones.
type App struct {
	Name string
	Root string // the outermost bin; "" is Name
	Args []string

	// Default runs when no command word is given or the word names no
	// command; it receives every argument. Without it, help is printed to
	// Stderr and Run exits 1. Default returns ErrNotFound to hand an
	// unknown word back to Run, which then reports it as NotFound does.
	Default func(ctx context.Context, args []string) error

	// Claims (WithClaims) says whether the fallback resolves args, the
	// whole argv. It is asked only when the first word exactly names a
	// Yield command; true runs the fallback instead of that command.
	Claims func(args []string) bool

	Path func() string // the PATH list; nil reads $PATH
	FS   FS            // nil is OS
	Env  []string      // an external's environment; nil is os.Environ()

	// Prefixes (WithPrefixes) are extra bin names whose <name>-<sub>
	// executables are discovered after the bin's own.
	Prefixes   []string
	Categories map[string]string // optional category overrides by canonical command name

	Stdin          io.Reader
	Stdout, Stderr io.Writer
	HelpText       string // optional application notes appended to help

	commands   []*Command
	catalog    *Catalog
	merged     bool         // the catalog holds merge's subcommands
	merge      *mergeSource // the external sharing this group's name
	invocation string       // spaced command path for declarative groups
}

// Option configures an App in New.
type Option func(*App)

// New builds an App named name over args. Root is taken from ODEN_ROOT
// when name is nested under it (an external run by an oden parent).
func New(name string, args []string, opts ...Option) *App {
	a := &App{Name: name, Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	for _, opt := range opts {
		opt(a)
	}
	if a.Root == "" {
		if root := a.getenv(EnvRoot); root != "" && strings.HasPrefix(name, root+"-") {
			a.Root = root
		}
	}
	return a
}

// WithPath fixes the PATH list discovery reads.
func WithPath(path string) Option { return func(a *App) { a.Path = func() string { return path } } }

// WithFS sets the file system discovery reads.
func WithFS(fsys FS) Option { return func(a *App) { a.FS = fsys } }

// WithEnv sets the environment externals start with.
func WithEnv(env []string) Option { return func(a *App) { a.Env = env } }

// WithStdio sets the streams commands read and write.
func WithStdio(in io.Reader, out, errOut io.Writer) Option {
	return func(a *App) { a.Stdin, a.Stdout, a.Stderr = in, out, errOut }
}

// WithPrefixes adds bin names whose <name>-<sub> executables on PATH are
// also this bin's commands, discovered after its own <bin>-<sub> and in
// the given order: with WithPrefixes("o"), mtm ai runs o-ai unless an
// mtm-ai is on PATH, which then shadows it. A Child does not inherit
// them.
func WithPrefixes(names ...string) Option {
	return func(a *App) { a.Prefixes = append(a.Prefixes, names...) }
}

// WithCategories assigns listing groups by canonical command name, including
// discovered externals. Unassigned commands remain visible under Other.
func WithCategories(categories map[string]string) Option {
	return func(a *App) { a.Categories = categories }
}

// WithDefault sets the handler for no command word or an unknown one.
func WithDefault(run func(ctx context.Context, args []string) error) Option {
	return func(a *App) { a.Default = run }
}

// WithClaims lets the app's fallback win over the commands marked Yield
// (the machine builtins state, catalog, commands, command) when the first
// word names one exactly and claims(args) is true for the whole argv, so a
// bin can read "state" as a verb without the builtin catching it. Without
// a claim, or when claims returns false, the command runs as before. Human
// builtins, app commands, prefix matches and the descendant rule are
// unaffected. claims must be a cheap predicate; the fallback itself (Default)
// runs once, afterwards, and sees the same args.
func WithClaims(claims func(args []string) bool) Option {
	return func(a *App) { a.Claims = claims }
}

// Child is the App for parent's command sub: named <parent>-<sub>, so its
// own externals are <parent>-<sub>-<word>, sharing the parent's root,
// PATH, file system, environment and streams.
func Child(parent *App, sub string, args []string) *App {
	return &App{
		Name: parent.Name + "-" + sub, Root: parent.RootName(), Args: args,
		Path: parent.Path, FS: parent.FS, Env: parent.Env,
		Stdin: parent.Stdin, Stdout: parent.Stdout, Stderr: parent.Stderr,
		HelpText: parent.HelpText,
	}
}

// RootName is the outermost bin of the chain.
func (a *App) RootName() string {
	if a.Root != "" {
		return a.Root
	}
	return a.Name
}

// Add registers inline commands, recording the caller as their Source
// when none is set.
func (a *App) Add(cmds ...Command) *App { return a.add(2, cmds...) }

func (a *App) add(skip int, cmds ...Command) *App {
	source := ""
	if _, file, line, ok := runtime.Caller(skip); ok {
		source = fmt.Sprintf("%s:%d", shortFile(file), line)
	}
	for _, c := range cmds {
		if c.Source == "" {
			c.Source = source
		}
		a.commands = append(a.commands, &c)
	}
	a.catalog, a.merged = nil, false
	return a
}

// inline is the commands in registration order for NewCatalog: the app's
// own first, then the builtins, so an app command named like a builtin
// owns the name (whenever it was added) and the builtin is its shadowed
// claim.
func (a *App) inline() []*Command {
	out := make([]*Command, 0, len(a.commands))
	for _, builtin := range []bool{false, true} {
		for _, c := range a.commands {
			if c.builtin == builtin {
				out = append(out, c)
			}
		}
	}
	return out
}

// shortFile is a source file's last directory and name.
func shortFile(file string) string {
	return filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file))
}

// Catalog is the merged inline and external commands, discovered on first
// use and cached until the next Add. A command group whose name an
// external also has adds that external's subcommands (merge.go).
func (a *App) Catalog() *Catalog {
	c := a.baseCatalog()
	if a.merge != nil && !a.merged {
		a.merged = true
		a.merge.mergeInto(a, c)
	}
	return c
}

// baseCatalog is the catalog without a merged external's subcommands:
// what an exact word resolves against before the external is asked.
func (a *App) baseCatalog() *Catalog {
	if a.catalog == nil {
		fsys := a.FS
		if fsys == nil {
			fsys = OS
		}
		found, shadowed := DiscoverAll(append([]string{a.Name}, a.Prefixes...), a.pathList(), fsys)
		a.catalog = NewCatalog(a.Name, a.inline(), found, shadowed)
		for _, e := range a.catalog.Entries {
			if category, ok := a.Categories[e.Name]; ok {
				e.Category = category
			}
		}
	}
	return a.catalog
}

func (a *App) pathList() string {
	if a.Path != nil {
		return a.Path()
	}
	return os.Getenv("PATH")
}

func (a *App) env() []string {
	if a.Env != nil {
		return a.Env
	}
	return os.Environ()
}

func (a *App) getenv(key string) string {
	for _, kv := range a.env() {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// ExitError carries a process exit code; Err (may be nil) is the cause.
// A nil Err means the message, if any, was already printed.
type ExitError struct {
	Code int
	Err  error
}

// Error returns the cause's message, or "exit status N" without one.
func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// Unwrap returns the cause, so errors.Is and errors.As see through it.
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode is the process exit code for err: 0 for nil, an ExitError's
// code, else 1.
func ExitCode(err error) int {
	var ee *ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.Code
	}
	return 1
}

// Run resolves the first argument and runs what it names with the rest:
// an inline command in process, an external with inherited streams. No
// word or an unknown one runs Default (or prints help and exits 1); an
// ambiguous prefix prints the candidates and exits 1.
func (a *App) Run(ctx context.Context) error {
	if len(a.Args) == 0 {
		return a.fallback(ctx, "")
	}
	if a.merge != nil && !a.merged {
		// An exact inline or PATH word outranks every merged subcommand:
		// run it without asking the external.
		if e := a.baseCatalog().Lookup(a.Args[0]); e != nil {
			if a.yields(e) {
				return a.fallback(ctx, a.Args[0])
			}
			return a.RunEntry(ctx, e, a.Args[1:])
		}
	}
	res := a.Catalog().Resolve(a.Args[0])
	switch {
	case res.Ambiguous:
		fmt.Fprintf(a.Stderr, "%s: %q is ambiguous; did you mean one of these?\n", a.helpName(), res.Query)
		a.List(a.Stderr, res.Candidates)
		return &ExitError{Code: 1}
	case !res.Resolved():
		return a.fallback(ctx, res.Query)
	}
	if res.Exact && a.yields(res.Entry) {
		return a.fallback(ctx, res.Query)
	}
	return a.RunEntry(ctx, res.Entry, a.Args[1:])
}

// yields reports whether e is an inline Yield command that the app's
// Claims takes over for the current argv.
func (a *App) yields(e *Entry) bool {
	return a.Claims != nil && e.Kind == KindInline && e.Command != nil && e.Command.Yield &&
		e.Sub == "" && a.Claims(a.Args)
}

func (a *App) fallback(ctx context.Context, word string) error {
	if a.Default != nil {
		err := a.Default(ctx, a.Args)
		if errors.Is(err, ErrNotFound) && word != "" {
			return a.NotFound(word)
		}
		return err
	}
	if a.merge != nil && a.merge.passThrough && word != "" {
		return a.runExternal(ctx, a.merge.path, a.Args, a.merge.parent)
	}
	if word == "" {
		fmt.Fprintf(a.Stderr, "usage: %s COMMAND [ARGS...]\n\ncommands:\n", a.helpName())
		a.List(a.Stderr, a.Catalog().Entries)
		return &ExitError{Code: 1}
	}
	return a.NotFound(word)
}

// RunEntry runs one catalog entry with args.
func (a *App) RunEntry(ctx context.Context, e *Entry, args []string) error {
	if e.Kind == KindInline {
		c := e.Command
		if c.Children != nil {
			child := Child(a, c.Name, args)
			child.invocation = a.helpName() + " " + c.Name
			child.Add(c.Children...)
			if claim := mergedClaim(e); claim != nil {
				child.merge = &mergeSource{path: claim.Source, name: filepath.Base(claim.Source), parent: a.Name}
			}
			if len(args) == 0 || len(args) == 1 && helpFlag(args[0]) {
				child.Help(a.Stdout)
				return nil
			}
			return child.Run(ctx)
		}
		if len(args) == 1 && helpFlag(args[0]) &&
			(a.invocation != "" || c.Usage != "" || c.Details != "" || c.Parameters != nil) {
			return a.commandHelp(a.Stdout, c)
		}
		if e.Command.Handle != nil {
			return e.Command.Handle(&Ctx{
				Context: ctx, App: a, Name: e.Command.Name, Args: args,
				Stdin: a.Stdin, Stdout: a.Stdout, Stderr: a.Stderr,
			})
		}
		return e.Command.Run(ctx, args)
	}
	if e.Sub != "" {
		parent := e.parent
		if parent == "" {
			parent = a.Name
		}
		return a.runExternal(ctx, e.Path, append([]string{e.Sub}, args...), parent)
	}
	return a.runExternal(ctx, e.Path, args, a.Name)
}

// externalEnv is the environment an external starts with: the App's plus
// ODEN_PARENT=parent and ODEN_ROOT.
func (a *App) externalEnv(parent string) []string {
	return append(append([]string{}, a.env()...), EnvParent+"="+parent, EnvRoot+"="+a.RootName())
}

func (a *App) runExternal(ctx context.Context, path string, args []string, parent string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.Stdin, a.Stdout, a.Stderr
	cmd.Env = a.externalEnv(parent)
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &ExitError{Code: ee.ExitCode()}
	}
	return err
}

// Report prints err as "prefix: err" to w, unless it is an ExitError with
// no cause (its message was already printed), and returns the exit code.
func Report(w io.Writer, prefix string, err error) int {
	var ee *ExitError
	if err != nil && !(errors.As(err, &ee) && ee.Err == nil) {
		fmt.Fprintf(w, "%s: %v\n", prefix, err)
	}
	return ExitCode(err)
}
