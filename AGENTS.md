# oden: agent guide

The lowest-level layer for subcommand-driven CLIs: inline commands plus
`<bin>-<sub>` executables on PATH, resolved by name, alias or unique prefix.
Standard library only. Single flat package at the repo root, `examples/hello`
as the reference consumer, `cmd/o` the family root bin. README.md holds the contract (naming, discovery,
ownership, resolution, exit codes, env vars); keep it in sync with the code.

## Plans and todos

Keep planning work in the family planning workspace, separate from this repository. Do not add a TODO.md or docs/plans/ to this repo.

## Where does X live

| Concern | File |
|---|---|
| `App`, options, `Child`, `Run`, external exec, `ExitError`/`ExitCode`/`Report` | `oden.go` |
| `Handler`, `Ctx`, `App.Cmd`, input helpers and `Ctx.Sub` | `handler.go` |
| `Catalog`, `Entry`, `Claim`, ownership and shadowing, `Resolve` | `catalog.go` |
| PATH scan, `FS`, `Discover`, `DiscoverAll` (extra prefixes), `Executable` | `discover.go` |
| Discovery memo and disk cache (PATH dir mtimes) | `discover_cache.go` |
| The `o` root bin (builtins + `o-<tool>`) | `cmd/o` |
| Merging a command group with the same-named external (probe, cache) | `merge.go` |
| Help listing | `help.go` |
| Opt-in `help`, `commands`, `which`, `catalog`, `state`, `command` | `builtins.go` |
| The schema-1 catalog answer and command rows (`CatalogDoc`, `CommandRows`) | `catalogdoc.go` |
| Runnable bin-to-bin delegation examples | `examples/delegation.go` |

## What oden owns

Discovery, ownership, resolution, dispatch and the help/catalog views of one
bin. It does not parse flags, has no TUI, and knows no application; global
flags belong to the app before it builds the `App`, and everything after the
command word is the command's.

## Invariants

- No app knowledge. No command name appears in the core except the opt-in
  builtins, which are ordinary `Command` rows added through `Add`.
- Every command is a row: an inline `Command` or a discovered `External`,
  merged into one `Catalog`. Help, `commands`, `which` and `Run` all read that
  one table; nothing re-derives it.
- Resolution rules are data, in one place (`Catalog.Resolve` and
  `NewCatalog`): exact name or alias, then unique prefix, outer beats nested;
  ownership inline name > external > inline alias; first registration and
  first on PATH win. Never special-case a name to change the order.
- Nothing is shadowed silently: every losing claim is recorded on the winner.
- A command group merges the same-named external (`Claim.Merged`); the
  external is asked for its subcommands only when that group runs a word
  that is not an exact inline or PATH child, and the answer is cached.
- Discovery and the environment are injectable (`WithPath`, `WithFS`,
  `WithEnv`, `WithStdio`); tests never depend on the developer's PATH.
- `Cmd` registers ordinary `Command` rows. `Handle` takes precedence over
  legacy `Run`; preserve both entry points through `RunEntry`.
- Input helpers open OS files lazily; `WithFS` is discovery-only. Readers
  own opened files, never stdin. See README for line limits and input rules.
- Externals get `ODEN_PARENT` and `ODEN_ROOT`; a child bin is `<bin>-<sub>`.
- Extra prefixes are opt-in (`WithPrefixes`) and always rank after the bin's
  own `<bin>-<sub>`; the loser is shadowed, never dropped. No bare-name
  lookup: a command is always `<prefix>-<sub>`, never an arbitrary PATH tool.
- `cmd/o` is the family root: builtins and `o-<tool>` externals only; the
  TUI host is `o-nabe`/`o-tui` on PATH. No TUI code in oden.
- Never bind function keys anywhere (examples and a future TUI included).

## Checks

```sh
gofmt -l . && go vet ./... && go test -race ./...
go test -run Golden -update   # after an intended help format change
```

## Releasing

Semver tags on `main`; consumers (ochazuke, then the family bins) require the tag.

1. Pick the version (`v0.x.y`: minor for API changes, patch for fixes).
2. Add a `## vX.Y.Z (DATE)` section at the top of CHANGELOG.md (above o-inventory's dated entries) with the highlights.
3. Run the checks with `GOWORK=off` and `go mod tidy` (no diff); commit and push.
4. `git tag -a vX.Y.Z -m "..."` on that exact commit, `git push origin vX.Y.Z`, `gh release create vX.Y.Z --notes ...`. Never move or delete a pushed tag; fix forward with the next patch.
5. Check the proxy: `GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod go list -m ofam.dev/oden@vX.Y.Z`.
6. Bump consumers: `go get ofam.dev/oden@vX.Y.Z && go mod tidy`, gate with `GOWORK=off`, commit and push.

Local development uses an untracked `go.work` in each consumer (`go work init . && go work use ../oden`), ignored via `.gitignore`; never commit a `replace` to a local path or a shared `~/proj/go.work`.
