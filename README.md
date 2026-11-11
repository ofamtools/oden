<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-animated-dark.svg">
    <img alt="oden" src="assets/logo-animated-light.svg" width="201">
  </picture>
  <br>
  <a href="https://github.com/ofamtools/oden/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/ofamtools/oden"></a>
  <a href="https://pkg.go.dev/ofam.dev/oden"><img alt="Go Reference" src="https://pkg.go.dev/badge/ofam.dev/oden.svg"></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/ofamtools/oden"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/ofamtools/oden"></a>
</p>

<p align="center">Subcommand dispatch for Go CLIs: a bin's inline commands and every <code>&lt;bin&gt;-&lt;sub&gt;</code> executable on PATH, in one table, resolved by name, alias or unique prefix.</p>

<p align="center"><img alt="The hello example: its help listing inline and external commands, a unique prefix running a command, an ambiguous prefix listing its candidates, a descendant run by name, which, a did-you-mean suggestion and a handler reading stdin" src="assets/demo.gif" width="720"></p>

The lowest layer for extensible, subcommand-driven CLI tools in Go: a bin's
inline commands merged with third-party executables on PATH, git- and
hiiro-style. No dependencies outside the standard library, no application
knowledge.

Module `ofam.dev/oden`; requires Go 1.26 or newer. It is the
outer shell of [ochazuke](https://github.com/ofamtools/ochazuke) apps,
of [matomeru](https://github.com/ofamtools/matomeru)'s `mtm` and of the
family's `o-*` tools ([below](#in-the-family)).

## Install the `o` command

`o` is the o-family's root command, and it lives in this repository:

```sh
go install ofam.dev/oden/cmd/o@latest
```

With it installed you don't need the dash: `o NAME ARGS...` runs the
`o-NAME` executable on your PATH with every argument passed through, so
`o gh` runs `o-gh`, `o md plan.md` runs `o-md plan.md` and `o lane ...`
runs `o-lane ...`. Install a tool the same way, then run it through `o`:

```sh
go install ofam.dev/o-gh@latest
o gh
```

`o` scans every PATH directory for `o-*` executables, and any word may be
shortened to a unique prefix (`o la` runs `o-lane` when nothing else starts
with `la`). `o` with no arguments, or `o help`, lists the tools it found
beside its builtins; `o which gh` prints what a word resolves to. See
[The `o` root](#the-o-root) for the full rule.

## Install the library

```sh
go get ofam.dev/oden@latest
```

## Usage

A whole bin: one inline command with an alias, oden's builtins, and every
`hello-<sub>` executable on PATH as a command of its own.

```go
// Command hello is a bin with one inline command and every hello-<sub> on PATH.
package main

import (
	"context"
	"os"

	"ofam.dev/oden"
)

func main() {
	app := oden.New("hello", os.Args[1:], oden.WithBuiltins())
	app.Cmd("greet", "say hello", func(c *oden.Ctx) error {
		return c.Printf("hello, %s\n", c.Arg(0, "world"))
	}, "hi")
	os.Exit(oden.Report(os.Stderr, "hello", app.Run(context.Background())))
}
```

Put an executable `hello-weather` on PATH and it is `hello weather`, with no
change to the bin. Every word resolves by exact name or alias, else by unique
prefix:

```console
$ hello gr Ada
hello, Ada
$ hello hi
hello, world
$ hello wea Tokyo
weather in Tokyo: sunny, 21C
$ hello h
hello: "h" is ambiguous; did you mean one of these?
  greet, hi         (inline)    say hello                      hello/main.go:13
  help, -h, --help  (inline)    list the commands by audience  oden@v0.1.1/builtins.go:21
```

[`examples/hello`](examples/hello) is the same bin with more handlers
(`echo`, `cat`, `lines`) and an external `bin/hello-web`; the demo above
records it ([docs/vhs](docs/vhs)).

### Handlers and input

`Cmd` registers a `Handler` with arguments, context, and the app's streams.
`Command{Run: ...}` remains supported. `Command.Handle` takes precedence
when both fields are set. Registration through `Cmd` uses the same catalog,
ownership rules, aliases, and caller-source tracking as `Add`.

```go
app.Cmd("cat", "concatenate files or stdin", func(c *oden.Ctx) error {
	in := c.Input()
	defer in.Close()
	_, err := io.Copy(c.Stdout, in)
	return err
})
```

| Input | Example | Handler access |
|---|---|---|
| Positional arguments | `hello greet Ada` | `c.Args`, `c.Arg(0, "world")` |
| Pipe | `printf 'hello\n' \| hello cat` | `c.Stdin` or `c.Input()` |
| Redirection | `hello cat < notes.txt` | Same stdin stream |
| File paths | `hello cat a.txt b.txt` | `c.Input()` |
| Files mixed with stdin | `printf 'middle\n' \| hello cat a.txt - b.txt` | `-` reads stdin in order |

`Open(paths...)` opens OS files lazily, one at a time, and reports failures
through `Read`. No paths means stdin. Close the reader when done; closing
never closes stdin. `WithFS` affects executable discovery, not input files.
`Text()` reads all input into memory. `Lines()` streams newline-stripped
lines and an optional final error, closing files even on an early `break`.
It uses `bufio.Scanner`'s default 64 KiB token limit. For larger lines, use
`Open` with your own scanner or buffered reader.

Input helpers concatenate bytes without inserting separators between files.
They consume stdin from its current position; repeated `-` does not rewind it.
Arguments are paths only when you call `Input`, `Text`, or `Lines`.
Parse flags yourself and pass the remaining paths to `Open` when needed.
No `Piped()` heuristic is exposed: pipes and redirects both work by reading
stdin, and a generic injected `io.Reader` has no portable terminal status.
Context cancellation is available through `c.Context`; it does not interrupt
a blocking file or stdin read automatically.

For an in-process command group, `Sub` inherits the app's discovery settings,
environment, root, and streams, but not extra prefixes:

```go
app.Cmd("admin", "administration", func(c *oden.Ctx) error {
	return c.Sub(oden.WithBuiltins()).
		Cmd("status", "show status", func(c *oden.Ctx) error {
			return c.Println("ready")
		}).Run(c)
})
```

#### Declarative command groups

Register child rows with `Command.Children` when a command has subcommands.
Oden owns their resolution, dispatch, and contextual help; no adapter-specific
help renderer is needed.

```go
app.Add(oden.Command{Name: "project", Children: []oden.Command{
	{Name: "list", Summary: "list projects", Handle: listProjects},
	{Name: "new", Summary: "create a project", Handle: newProject,
		Usage: "[--name NAME]",
		Parameters: []oden.Parameter{
			{Name: "name", Type: "string", Description: "Project name", Required: true},
		}},
}})
```

`BIN project`, `BIN project -h`, and `BIN project --help` list that group's
commands, descriptions, and parameter metadata without running a handler.
`BIN project new --help` and builtin `BIN help project new` describe one leaf.
Unknown child commands list the current group's candidates, not unrelated
root commands. Aliases, unique prefixes, ownership, and `<bin>-project-<sub>`
external discovery use the ordinary child catalog.

`Children: nil` is a leaf; a non-nil empty slice declares an empty group.
Groups dispatch their children; `Run` / `Handle` belong on leaves. An omitted
group summary is derived from its child names. Optional `Usage`, `Details`,
and `Parameters` describe leaf help, while `App.HelpText` supplies shared
application notes inherited by child groups. Parameter order is the order
registered; `Choices`, `Source`, and `Free` describe accepted values. These
fields are display metadata: oden does not parse or validate application flags.
Existing leaf handlers without help metadata retain ownership of their arguments.

Default help (`BIN`, `-h`, `--help`, `help`) lists the bin's own commands. A
command with `Namespace` set belongs to a namespace that is not the bin's own;
the namespace is one line (the `NamespaceHead` command's names and summary, else
a member count) saying `BIN help --all` or `BIN NS` lists it. `Hidden` commands
and shadow lines for namespace members are left out, and `App.HelpBrief`
replaces `HelpText` in that listing. `BIN help --all` (or `-h --all`,
`--help --all`, `App.HelpAll`) lists everything. `commands`, `catalog` and
`state` are unchanged.

### One oden bin delegates to another

[`examples/delegation.go`](examples/delegation.go) contains runnable apps
selected by executable name. Real projects can keep each app in its own
package. Build the example executables into a temporary directory:

```sh
bins=$(mktemp -d)
for name in toolbox toolbox-files toolbox-files-text desk; do
	go build -o "$bins/$name" ./examples/delegation.go
done
export PATH="$bins:$PATH"
```

#### A parent delegates to a child bin

`toolbox` has no inline `files` handler. Discovery finds `toolbox-files`,
which is another oden app with an inline `info` handler:

```sh
toolbox files info Ada
# bin=toolbox-files parent=toolbox root=toolbox arg=Ada
toolbox fi info Ada
# Same result through a unique prefix.
```

#### Three bins form a command chain

`toolbox-files` discovers `toolbox-files-text`. Each bin consumes its own
command word and forwards the rest, together with stdin, stdout, and stderr:

```sh
printf 'hello\n' | toolbox files text show
# hello
toolbox files text show README.md
# Prints README.md.
toolbox files text show < README.md
# Prints the same input through stdin.
```

#### An unrelated bin shares another family's commands

`desk` opts into `WithPrefixes("toolbox")`. It can dispatch `files` to the
same `toolbox-files` executable without a wrapper or duplicate handler:

```sh
desk files info Ada
# bin=toolbox-files parent=desk root=toolbox-files arg=Ada
printf 'shared\n' | desk files text show
# shared
```

The environment contains `ODEN_ROOT=desk` on the first shared-prefix hop.
`New` adopts that root only when the executable name starts with `desk-`.
Here it does not, so `toolbox-files` starts its own root, as the output shows.
An installed `desk-files` wins over `toolbox-files`; the catalog records the
losing executable as shadowed.

## Concepts

The rules every oden bin follows: how a command is named and found, who owns a
word, how a word resolves, and what runs. Help, `commands`, `which` and `Run`
all read one table of rows built by these rules.

### The contract

**Naming.** Any executable named `<bin>-<sub>` on PATH is the command `sub`
of `bin`: put `mtm-web` on PATH and `mtm web ARGS...` runs it, as `git-web`
makes `git web`. Nothing in the bin changes.

**Discovery.** PATH is read in order (an empty entry is `.`, a repeated
directory is read once at its first place). A candidate is a regular file,
or a link to one, with an executable bit; directories and non-executables
are skipped. The first `<bin>-<sub>` on PATH wins; later ones are recorded as
shadowed. Discovery runs once, on first use, and is cached until the next
`Add` or `Cmd`. The PATH list and the file system are injectable (`WithPath`,
`WithFS`). On the real file system the result is also remembered across
apps and processes (`$XDG_CACHE_HOME/oden/discover.json`, checked against each
PATH directory's modification time); `ODEN_DISCOVER_CACHE=off` turns the disk
part off.

**Extra prefixes** (opt-in, `WithPrefixes("o")`): more bin names whose
`<name>-<sub>` executables are also this bin's commands, discovered after
the bin's own `<bin>-<sub>` (every PATH directory for the bin's prefix, then
every directory per extra prefix, in the given order). So `mtm` built with
`WithPrefixes("o")` runs `o-ai` for `mtm ai`, and an `mtm-ai` anywhere
on PATH wins and lists `o-ai` as shadowed. Entries carry their `prefix`
(JSON) and help shows the path. The descendant rule below is unchanged
across prefixes; a `Child` does not inherit extra prefixes.

**Ownership** of a token, highest first:

1. an inline command's name (the first registration wins)
2. a namespace head's alias equal to its `Namespace` (the first registration wins)
3. an external's name (the first on PATH wins)
4. an ordinary inline alias (the first registration wins)

Every losing claim stays on the winner as a `shadows ...` line in help and in
`commands --json`, so nothing is hidden silently.

An explicitly registered namespace owns its word over a same-named executable
discovered on PATH. For example, a `NamespaceHead` named `lab.` with
`Namespace: "lab"` and alias `lab` keeps `BIN lab record export` inside the
registered namespace rather than running an unrelated or older `BIN-lab`.
The external stays shadowed, not merged into the namespace. An exact inline
command named `lab` still wins whether registered before or after the namespace;
`lab.` remains available to reach the namespace itself.

**Resolution** of the first word: an exact name or alias, else the one
command with a name or alias starting with the word (`mtm ta` is `task` when
nothing else starts with `ta`). Descendants never make a prefix ambiguous:
a candidate named `<other>-...` where `<other>` is also a candidate is that
candidate's child and is dropped, so when every candidate equals the
shortest one or starts with `<shortest>-`, the shortest wins (`horror bio`
is `biographies` even though `horror-biographies-freddie-kruger` is also
on PATH and listed as `biographies-freddie-kruger`, which, as with git, can
be run directly). Any other set of several candidates is ambiguous
(`horror b` with `biographies` and `books`). The rule is the same for inline
and external commands, and is hiiro's
(`~/proj/hiiro/lib/hiiro/catalog.rb:56-57`: prefix matches are collected by
`token.start_with?(word)`, then a match whose name starts with
`"<other match>-"` is rejected).

**Merge and shadow** (command groups). An inline command group `NAME`
(`Children`) and an external `<bin>-NAME` (or `<prefix>-NAME` through
`WithPrefixes`) on PATH are one group, not a winner and a loser: `BIN NAME
WORD ...` resolves `WORD` over the group's children (inline, and
`<bin>-NAME-<sub>` on PATH as for any child) together with the external's
subcommands. The order is unchanged: an exact name or alias first, inline
before the external (a name on both sides runs the inline one), then a
unique prefix over the union with the descendant rule. A prefix matching
several candidates across both sides is ambiguous and lists them with
their source. An external subcommand runs as `<bin>-NAME SUB ARGS...` with
the usual `ODEN_PARENT=<bin>` and `ODEN_ROOT`. The group's help lists the
union, each row with its source (`noun` or file:line inline, the
external's file name otherwise); an external subcommand that loses to an
inline name stays on the inline row as `shadows external <bin>-NAME SUB`
and is still reachable directly as `<bin>-NAME SUB`. The parent's help
shows the group row `merges external PATH`, and `commands` marks that
claim `"merged": true`. `subcommand not found` suggestions search both
sides.

Oden learns the external's subcommands by asking it, without any declared
dependency: `<bin>-NAME catalog` (an ochazuke catalog's nouns; for a plain
oden bin's `command` noun, its `command list` rows), else `<bin>-NAME
commands --json`, each with no stdin, the external's environment and a
2-second timeout. The answer, an empty one too, is cached in
`$XDG_CACHE_HOME/oden` (else `~/.cache/oden`), keyed by the executable's
path, size and modification time, so a rebuilt or touched external is
asked again and an unchanged one never is. An external that answers
nothing usable (a shell script) is pass-through: every word that does not
resolve to an inline child runs `<bin>-NAME WORD ...` unchanged, and the
group's help says `Other words run <bin>-NAME (PATH).` The external is
never run while a word resolves exactly (inline or a PATH child), nor for
a group that has no same-named external, nor for anything outside a
group. Only the first external on PATH merges; later ones stay shadowed.
A non-group inline command and a same-named external keep the plain rule
below: the inline command owns the word and the external is shadowed.
Merging is one level deep: a nested inline group merges its own
`<bin>-NAME-<sub>` external, not the parent external's verbs, and
`Ctx.Sub` groups (handler-built) do not merge.

**Not found.** A word with no exact and no prefix match names nothing. Run
prints to stderr `BIN: command "WORD" not found` (below the root, such as in
a command group, `BIN GROUP: subcommand "WORD" not found`), then, when
there are any, a short `did you mean: A, B` line of at most three commands
(first those whose name or alias is a prefix of the word, `tasks` suggests
`task`, then those within a small edit distance: one insertion, deletion,
substitution or adjacent swap for a word of up to five letters, else two)
and a `(BIN --help lists every command)` pointer; with no suggestion, the
level's help (the same listing as `BIN help` or a bare group). It exits 1. Suggestions are never run: `tasks` does not resolve to `task`.
`App.Suggest(word)` returns the suggestions and `App.NotFound(word)` prints
the report and returns the exit error.

**Running.** An inline command gets the remaining words. An external is
executed with the remaining words, the App's stdio, and the environment plus
`ODEN_PARENT=<bin>` and `ODEN_ROOT=<outermost bin>`. No word, or a word that
names nothing, runs the `Default` handler with every argument; without one,
no word prints help to stderr and exits 1, and an unknown word is reported
as above. A `Default` that only wants the no-word case (a TUI) returns
`oden.ErrNotFound` for words, and Run reports the unknown word as if there
were no `Default`. An ambiguous word prints the candidates to stderr and
exits 1, even with a `Default`.

**Yielding builtins.** The machine builtins (`state`, `catalog`, `commands`,
`command`) are marked `Yield`. With `oden.WithClaims(func(args []string) bool)`
(or `App.Claims`), when the first word exactly names a `Yield` command and
`claims(args)` is true for the whole argv, the fallback (`Default`, else the
not-found report) runs instead, so a bin whose `Default` reads a first word
as a verb (`o-lane state` meaning noun `lane`, verb `state`) is not caught by
the builtin. Claims is a predicate only; `Default` runs once, afterwards.
Without a claim the builtin runs as before, so the machine commands stay
reachable whenever the app does not take the word. Human builtins (`help`,
`info`, `version`, `which`), app commands, prefix matches and the descendant
rule never yield. oden has no separate namespace for builtins (no
`oden.state`); `Command.Yield` is data an app may set on its own commands too.

**Nesting.** `oden.Child(app, "ai", args)` is the App named `mtm-ai`, so it
resolves `mtm-ai-skills`; an external `mtm-ai` that is itself an oden app
gets its root from `ODEN_ROOT`. `mtm ai skills` → `mtm-ai skills` →
`mtm-ai-skills`; `horror bio fre` runs `horror-biographies fre`, which (an
oden app named `horror-biographies`) runs `horror-biographies-freddie-kruger`.

**Exit codes.** `Run` returns an error; `ExitCode(err)` is 0 for nil, the
code of an `*ExitError` (an external's own exit status, 1 for help and
ambiguity), else 1. `Report(w, prefix, err)` prints the error unless it is
an `ExitError` with no cause, and returns the code.

**Builtins** (opt-in, `WithBuiltins()`): usage separates commands by audience.
Human-facing commands print readable text; machine-facing commands retain
their established JSON formats without requiring an output flag.

| Audience | Invocation | Output |
|---|---|---|
| Human | `help`, `-h`, `--help` | Commands grouped by audience and optional domain category, with kind, summary, source and shadowed claims |
| Human | `info [COMMAND]` | Bin information, or a command's summary, audience, category, kind, source and aliases |
| Human | `version` | Embedded module version, full commit SHA, commit time, dirty-build flag, Go version and module path |
| Human | `which WORD` | `name kind source` for the resolved command |
| Machine | `commands` | Entry array: name, kind, aliases, summary, category, audience, source, prefix, shadowed |
| Machine | `catalog` | ochazuke/cli schema 1: command noun with list, show and state; no bindings or pages |
| Machine | `command list` | Command row array |
| Machine | `command show NAME` | One command row |
| Machine | `state`, `command state` | One command row per JSON Line |

`info COMMAND`, `which WORD` and `command show NAME` resolve names, aliases
and unique prefixes. A row has `noun`, `id`, `label` and `data`; data includes
kind, path, source, aliases, shadows, prefix and optional summary, category
and audience. Build info supplies the catalog version. `WriteCatalog` and
`WriteState` remain explicit JSON APIs.

Machine commands preserve their original argument handling: `commands`,
`catalog`, `state`, `command list` and `command state` ignore trailing
arguments, including the host convention `--json '{}' --yes`. `command show`
still takes exactly one name, and `which` exactly one word. No `-j` is
required, no `-H` / `--human` mode exists, and output never changes based on
whether stdout is a terminal. Use `help` rather than changing a machine
command's output format.

`info -h` / `info --help` prints its usage; `info -- -h` describes the help
alias. Invalid info arguments exit 2; unresolved names exit 1. Existing
builtins retain their prior usage-error exit behavior.

**Version provenance.** `version` is included by `WithBuiltins()`, so `o`
and applications using the builtins acquire it when rebuilt against this
oden version. App registrations can override it like any other builtin.
The command reads Go's embedded `runtime/debug.ReadBuildInfo`, not the
current directory, repository, or network. `Modified` describes the main
module's source repository at build time, not its current status. It does
not report dirty source in locally replaced dependency repositories.

Use a normal package build with VCS stamping enabled to include the commit,
for example `go build -buildvcs=true -o ~/bin/o ./cmd/o`. The version is
the embedded main-module version, including `(devel)` or a pseudo-version
when that is how the binary was built. Builds with no VCS metadata, such as
`-buildvcs=false` or some `go run` builds, print `unknown` for the commit,
time and modified flag rather than claiming a clean or released build.
`version -h` / `version --help` prints usage; unexpected arguments exit 2.
No output-format flag is needed.

**Audience.** Set `Command.Audience` to `AudienceMachine` for a structured
machine endpoint or `AudienceHuman` for readable output. Unspecified
commands, including discovered externals, are presented as human-facing.
Audience is presentation metadata, not an output serializer or dispatch
rule. An app command overriding a builtin inherits that builtin's audience
unless it explicitly sets one. Thus an overridden runtime `catalog` stays
in the machine group without changing its handler. The app still owns the
name whenever added, and the builtin remains a shadowed claim.

**Family flag convention.** Give long flags single-letter aliases from
`a-zA-Z` unless every available letter in that command's scope is already
assigned. Reserve `-h` for help. Human commands default to readable output;
documented machine endpoints retain stable structured output. Oden does not
parse arbitrary handlers' or external processes' flags. Ochazuke's existing
`--json OBJECT` input protocol is unchanged.

**Categories.** Audience and domain category are independent. Set
`Command.Category` for inline commands, or use
`WithCategories(map[string]string{"task": "Work", "web": "Network"})` to
assign or override groups by canonical name, including discovered externals.
`Cmd` users can supply the same option. Within each audience, help sorts
category headings alphabetically, keeps commands sorted within each group,
and retains uncategorized commands under `Other`. Without categories a group
stays flat. Apps with no machine commands retain the plain `commands:`
listing. JSON entries and command rows retain optional category/audience
metadata. Neither affects resolution.

Oden does not execute discovered tools to collect metadata, except to
learn a merged external's subcommands (see Merge and shadow). External
categories must be assigned by the owning app. `Child` does not inherit
category assignments because its command names belong to a different scope.

```
usage: mtm COMMAND [ARGS...]

commands:
  task, t  (inline)    tasks     app.go:10
           shadows external /bin/mtm-task
  web      (external)            /bin/mtm-web
           shadows external /opt/mtm-web
```

## API

The signatures below list the consumer methods. `go doc -all .` also shows
every exported data field and its documentation.

```go
type Handler func(*Ctx) error
type Ctx struct {
	context.Context
	App *App
	Name string
	Args []string
	Stdin io.Reader
	Stdout, Stderr io.Writer
}
type Option func(*App)
func New(name string, args []string, opts ...Option) *App
func WithBuiltins() Option
func WithDefault(run func(context.Context, []string) error) Option
func WithClaims(claims func(args []string) bool) Option
func WithPrefixes(names ...string) Option
func WithCategories(categories map[string]string) Option
func WithPath(path string) Option
func WithFS(fsys FS) Option
func WithEnv(env []string) Option
func WithStdio(in io.Reader, out, errOut io.Writer) Option
func Child(parent *App, sub string, args []string) *App
func (a *App) Cmd(name, summary string, run Handler, aliases ...string) *App
func (a *App) Add(cmds ...Command) *App
func (a *App) Catalog() *Catalog
func (a *App) RootName() string
func (a *App) Run(ctx context.Context) error
func (a *App) RunEntry(ctx context.Context, e *Entry, args []string) error
func (a *App) Help(w io.Writer)
func (a *App) List(w io.Writer, entries []*Entry)
func (a *App) NotFound(word string) error // reports an unknown word; exit 1
func (a *App) Suggest(word string) []string
var ErrNotFound error // returned by Default to defer an unknown word to Run
func (c *Ctx) Arg(i int, def string) string
func (c *Ctx) Open(paths ...string) io.ReadCloser
func (c *Ctx) Input() io.ReadCloser
func (c *Ctx) Text() (string, error)
func (c *Ctx) Lines() iter.Seq2[string, error]
func (c *Ctx) Printf(f string, a ...any) error
func (c *Ctx) Println(a ...any) error
func (c *Ctx) Usage(synopsis string) error // prints usage; exit code 2
func (c *Ctx) Sub(opts ...Option) *App
func NewCatalog(bin string, inline []*Command, externals, shadowed []External) *Catalog
func (c *Catalog) Resolve(word string) Resolution
func (c *Catalog) Lookup(token string) *Entry
func (r Resolution) Resolved() bool
func Discover(bin, path string, fsys FS) (found, shadowed []External)
func DiscoverAll(bins []string, path string, fsys FS) (found, shadowed []External)
func PathDirs(path string) []string
func Executable(fsys FS, path string) bool
func ExitCode(err error) int
func Report(w io.Writer, prefix string, err error) int
func (e *ExitError) Error() string
func (e *ExitError) Unwrap() error
```

Exported data types and values:

| Name | Consumer-facing data |
|---|---|
| `Command` | `Name`, `Aliases`, `Summary`, `Category`, `Audience`, `Yield`, `Children`, `Usage`, `Details`, `Parameters`, legacy `Run`, `Handle`, `Source` |
| `Parameter` | `Name`, `Type`, `Description`, `Required`, `Choices`, `Source`, `Free` |
| `App` | `Name`, `Root`, `Args`, `Default`, `Claims`, `Path`, `FS`, `Env`, `Prefixes`, `Stdin`, `Stdout`, `Stderr`, `HelpText` |
| `Catalog` | `Bin`, sorted `Entries` |
| `Entry` | `Name`, `Kind`, `Aliases`, `Summary`, `Source`, `Prefix`, `Shadowed`, `Sub` (a merged external's subcommand), `Command`, `Path` |
| `Claim` | `Name`, `Kind`, `Source`, `AliasOf`, `Merged` |
| `External` | `Name`, `Path`, `Prefix` |
| `Resolution` | `Query`, `Entry`, `Exact`, `Ambiguous`, `Candidates` |
| `Kind` | `KindInline`, `KindExternal` |
| `FS` | `ReadDir(string) ([]fs.DirEntry, error)`, `Stat(string) (fs.FileInfo, error)` |
| `OS` | OS-backed discovery filesystem |
| `ExitError` | `Code int`, `Err error` |
| `EnvParent`, `EnvRoot` | Environment keys `ODEN_PARENT`, `ODEN_ROOT` |

## In the family

oden is the bottom layer of the o-family. It knows no application and no TUI;
the layers above it are ordinary consumers:

| Consumer | How it uses oden |
|---|---|
| [ochazuke](https://github.com/ofamtools/ochazuke) | The TUI framework builds on oden for CLI resolution: an app's nouns become inline commands of an oden `App`. |
| [matomeru](https://github.com/ofamtools/matomeru) (`mtm`) | An oden app with `WithPrefixes("o")`: `mtm-<sub>` bins first, then the family's `o-<sub>` tools, so `mtm ai` runs `o-ai`. |
| `o` (`cmd/o`) | The family root: oden's builtins and every `o-<tool>` on PATH. |
| The `o-*` tools (`o-ai`, `o-proj`, `o-json`, `o-md`, ...) | Each is an oden app on PATH, so `o ai`, `o proj` and `mtm ai` reach it with no registration. |

### The `o` root

`cmd/o` is the family root bin (oden, ochazuke): an oden app named `o` with
the builtins and nothing else built in. Every word follows the normal rule,
so a tool joins the family by installing as `o-<tool>`:

| Word | Resolves to, highest first |
|---|---|
| `o help`, `o info [W]`, `o version`, `o which W`; `o commands`, `o catalog`, `o state`, `o command list\|show\|state` | inline builtins; first group human-facing, second group JSON-first |
| `o proj new x` | external `o-proj new x` (`ODEN_PARENT=o`) |
| `o pr new x` | the same, by unique prefix |

Install: `go install ofam.dev/oden/cmd/o@latest`, or `go build -o ~/bin/o ./cmd/o` from a checkout. Another app shares the family's
tools with `WithPrefixes("o")`, as `mtm` does (`mtm ai` runs `o-ai`).

### TUI mode and ochazuke

Not built yet; this is the intended seam. oden stays the outer shell and
knows no TUI: an oden app hands its `Catalog` (plain data: entries with
name, kind, aliases, summary, source and shadowed claims) to an
ochazuke-based TUI that defines a noun `command` whose list rows are the
catalog entries, with verbs such as `run` (calls `RunEntry`, an external
under a terminal instance) and `which`, so the TUI is one more consumer of
the same table that help and `commands` read. In the other direction an
ochazuke app mounts oden as its outer shell, as matomeru's `mtm` does: oden
resolves the first word, ochazuke's nouns are registered as inline commands
that delegate to the app's NOUN VERB shell, and `Default` runs the TUI when
no word is given.

## Checks

```sh
gofmt -l . && go vet ./... && go test -race ./...
go run ./examples/hello help
go run ./cmd/o help
```

## Logo and demo

The logo is the Kushi: konnyaku, daikon and chikuwa threaded on one bamboo skewer, as subcommands sit on one bin, in oden's dashi amber. `python3 docs/logo/build.py` writes `assets/icon.svg`, `assets/logo-light.svg` and `assets/logo-dark.svg`, with the wordmark drawn in the family's lettering (ochazuke's and matomeru's; `--check` confirms the glyphs still redraw theirs, `--preview DIR` renders the icon at 16, 32 and 128 px). `vhs docs/vhs/demo.tape` records `assets/demo.gif` in a throwaway directory; see [docs/vhs/README.md](docs/vhs/README.md).

MIT license.
