package oden

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ExtField describes a schema field or command parameter in Matomeru's
// extension manifest wire format.
type ExtField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
	RefKinds    []string `json:"refKinds,omitempty"`
}

// ExtStyleRule describes a declarative row or cell styling rule on an
// extension schema.
type ExtStyleRule struct {
	When     string `json:"when"`
	Target   string `json:"target,omitempty"` // "" or "row" for whole-row, or column key
	Tone     string `json:"tone,omitempty"`
	Icon     string `json:"icon,omitempty"`
	IconTone string `json:"iconTone,omitempty"`
}

// ExtSchema describes one noun schema in an extension manifest.
type ExtSchema struct {
	Kind           string         `json:"kind"`
	Title          string         `json:"title"`
	Plural         string         `json:"plural"`
	Fields         []ExtField     `json:"fields"`
	DefaultColumns []string       `json:"defaultColumns,omitempty"`
	SummaryField   string         `json:"summaryField,omitempty"`
	UniqueFields   []string       `json:"uniqueFields,omitempty"`
	StyleRules     []ExtStyleRule `json:"styleRules,omitempty"`
}

// ExtCommandExec describes a declarative process invocation for an extension
// command.
type ExtCommandExec struct {
	Argv         []string          `json:"argv"`
	Title        string            `json:"title,omitempty"`
	Dir          string            `json:"dir,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Foreground   bool              `json:"foreground,omitempty"`
	RefreshKinds []string          `json:"refreshKinds,omitempty"`
}

// ExtCommandGuard describes a declarative precondition for an extension
// command.
type ExtCommandGuard struct {
	When          string `json:"when"`
	RefuseMessage string `json:"refuseMessage,omitempty"`
	HideWhenFalse bool   `json:"hideWhenFalse,omitempty"`
}

// ExtCommandConfirm describes a declarative confirmation prompt for an
// extension command.
type ExtCommandConfirm struct {
	Title   string `json:"title,omitempty"`
	Message string `json:"message"`
	Label   string `json:"label,omitempty"`
	Tone    string `json:"tone,omitempty"`
}

// ExtCommandFeedback describes declarative status messages for an extension
// command.
type ExtCommandFeedback struct {
	Running string `json:"running,omitempty"`
	Success string `json:"success,omitempty"`
	Failure string `json:"failure,omitempty"`
}

// ExtCommand describes one command entry in an extension manifest.
type ExtCommand struct {
	ID          string              `json:"id"`
	Description string              `json:"description,omitempty"`
	Params      []ExtField          `json:"params,omitempty"`
	Kinds       []string            `json:"kinds,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Key         string              `json:"key,omitempty"`
	KeyHint     string              `json:"keyHint,omitempty"`
	Exec        *ExtCommandExec     `json:"exec,omitempty"`
	Guard       *ExtCommandGuard    `json:"guard,omitempty"`
	Confirm     *ExtCommandConfirm  `json:"confirm,omitempty"`
	Feedback    *ExtCommandFeedback `json:"feedback,omitempty"`
	Meta        map[string]any      `json:"meta,omitempty"`
}

// ExtManifest is the wire shape printed by `TOOL catalog` for Matomeru's
// extension protocol.
type ExtManifest struct {
	Name     string         `json:"name"`
	Version  string         `json:"version,omitempty"`
	Schemas  []ExtSchema    `json:"schemas,omitempty"`
	Commands []ExtCommand   `json:"commands,omitempty"`
	Bindings []any          `json:"bindings,omitempty"`
	Views    map[string]any `json:"views,omitempty"`
}

// ExtRecord is one record returned by `TOOL query`.
type ExtRecord struct {
	Kind            string          `json:"kind"`
	ID              string          `json:"id"`
	Fields          map[string]any  `json:"fields"`
	ReadOnly        map[string]bool `json:"readOnly,omitempty"`
	ArchiveStatus   string          `json:"archiveStatus,omitempty"`
	StarStatus      string          `json:"starStatus,omitempty"`
	ChangeSetNumber int64           `json:"changeSetNumber,omitempty"`
	CreatedAt       time.Time       `json:"createdAt,omitzero"`
	UpdatedAt       time.Time       `json:"updatedAt,omitzero"`
}

// ExtQueryResult is the wire shape printed by `TOOL query`.
type ExtQueryResult struct {
	Items     []ExtRecord `json:"items"`
	Total     int         `json:"total"`
	FilterErr string      `json:"filterErr,omitempty"`
}

// Capabilities is the wire shape printed by `TOOL capabilities --json`.
type Capabilities struct {
	Name         string   `json:"name,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities"`
	Commands     []string `json:"commands"`
}

// AddExtension registers the standard family extension protocol commands on a:
//   - catalog: prints manifest as JSON (overrides oden's builtin catalog via normal inline precedence)
//   - query: runs queryFn(ctx, kind, args) and prints ExtQueryResult as JSON
//   - capabilities: prints Capabilities (JSON with --json / --format json, or human table otherwise)
//
// Oden's other builtins (commands, command, help, help --all, which, version)
// continue working unchanged alongside AddExtension.
func (a *App) AddExtension(manifest ExtManifest, caps Capabilities, queryFn func(ctx context.Context, kind string, args []string) (ExtQueryResult, error)) *App {
	return a.add(2,
		Command{
			Name:     "catalog",
			Audience: AudienceMachine,
			Summary:  "print the extension catalog manifest as JSON",
			Handle: func(c *Ctx) error {
				doc := manifest
				if doc.Name == "" {
					doc.Name = a.Name
				}
				if doc.Version == "" {
					doc.Version = buildVersion()
				}
				return c.WriteJSON(doc)
			},
		},
		Command{
			Name:     "query",
			Audience: AudienceMachine,
			Summary:  "query extension records as JSON",
			Usage:    "[--kind KIND] [QUERY...]",
			Parameters: []Parameter{
				{Name: "kind", Type: "string", Description: "Filter records by schema kind"},
			},
			Handle: func(c *Ctx) error {
				var kind string
				var rest []string
				for i := 0; i < len(c.Args); i++ {
					arg := c.Args[i]
					if arg == "--" {
						rest = append(rest, c.Args[i+1:]...)
						break
					}
					if arg == "--kind" && i+1 < len(c.Args) {
						i++
						kind = c.Args[i]
						continue
					}
					if v, ok := strings.CutPrefix(arg, "--kind="); ok {
						kind = v
						continue
					}
					rest = append(rest, arg)
				}
				var res ExtQueryResult
				if queryFn != nil {
					var err error
					res, err = queryFn(c.Context, kind, rest)
					if err != nil {
						return err
					}
				}
				if res.Items == nil {
					res.Items = []ExtRecord{}
				}
				if res.Total == 0 && len(res.Items) > 0 {
					res.Total = len(res.Items)
				}
				return c.WriteJSON(res)
			},
		},
		Command{
			Name:     "capabilities",
			Audience: AudienceMachine,
			Summary:  "print extension capabilities (--json for JSON)",
			Usage:    "[--json|--format json|table]",
			Handle: func(c *Ctx) error {
				p, err := c.Parse(nil)
				if err != nil {
					return err
				}
				out := caps
				if out.Name == "" {
					out.Name = a.Name
				}
				if out.Version == "" {
					if manifest.Version != "" {
						out.Version = manifest.Version
					} else {
						out.Version = buildVersion()
					}
				}
				if out.Capabilities == nil {
					out.Capabilities = []string{}
				}
				if len(out.Commands) == 0 {
					entries := a.Catalog().Entries
					cmds := make([]string, 0, len(entries))
					for _, e := range entries {
						cmds = append(cmds, e.Name)
					}
					out.Commands = cmds
				}
				if p.Format() == "json" {
					return c.WriteJSON(out)
				}
				rows := [][]string{
					{"name", out.Name},
					{"version", out.Version},
					{"summary", out.Summary},
					{"capabilities", strings.Join(out.Capabilities, ", ")},
					{"commands", strings.Join(out.Commands, ", ")},
				}
				if err := c.WriteTable([]string{"field", "value"}, rows); err != nil {
					return fmt.Errorf("capabilities: %w", err)
				}
				return nil
			},
		},
	)
}
