package oden

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Parsed holds the result of ParseFlags: flag values (with defaults applied),
// explicit presence tracking, and positional arguments.
type Parsed struct {
	values     map[string][]string // canonical name -> values (explicit or default)
	present    map[string]bool     // canonical name -> explicitly passed on CLI
	aliases    map[string]string   // alternate flag name -> canonical name
	positional []string
	format     string
}

func (p Parsed) canonical(name string) string {
	key := strings.TrimLeft(name, "-")
	if c, ok := p.aliases[key]; ok {
		return c
	}
	return key
}

// String returns the last value for name (or its default, or "").
func (p Parsed) String(name string) string {
	vals := p.values[p.canonical(name)]
	if len(vals) == 0 {
		return ""
	}
	return vals[len(vals)-1]
}

// Strings returns a copy of all values for name (or its default, or nil).
func (p Parsed) Strings(name string) []string {
	vals := p.values[p.canonical(name)]
	if len(vals) == 0 {
		return nil
	}
	return slices.Clone(vals)
}

// Has reports whether name was explicitly passed on the command line.
func (p Parsed) Has(name string) bool {
	return p.present[p.canonical(name)]
}

// Bool returns the boolean value for name (false if unset).
func (p Parsed) Bool(name string) bool {
	v, err := strconv.ParseBool(p.String(name))
	if err != nil {
		return false
	}
	return v
}

// Int returns the integer value for name (0 if unset).
func (p Parsed) Int(name string) int {
	v, err := strconv.Atoi(p.String(name))
	if err != nil {
		return 0
	}
	return v
}

// Float returns the float64 value for name (0 if unset).
func (p Parsed) Float(name string) float64 {
	v, err := strconv.ParseFloat(p.String(name), 64)
	if err != nil {
		return 0
	}
	return v
}

// Positional returns a copy of the positional arguments.
func (p Parsed) Positional() []string {
	return slices.Clone(p.positional)
}

// Arg returns the i-th positional argument, or fallback when i is out of bounds.
func (p Parsed) Arg(i int, fallback string) string {
	if i < 0 || i >= len(p.positional) {
		return fallback
	}
	return p.positional[i]
}

// Format returns the requested output format ("json" or "table", defaulting to "table").
func (p Parsed) Format() string {
	if p.format != "" {
		return p.format
	}
	return "table"
}

// ParseFlags parses args against params plus standard global helpers
// (--format json|table and --json shorthand). Flags may be interleaved with
// positional arguments; "--" stops flag parsing and treats all remaining
// arguments as positional.
func ParseFlags(args []string, params []Parameter) (Parsed, error) {
	out := Parsed{
		values:  map[string][]string{},
		present: map[string]bool{},
		aliases: map[string]string{},
		format:  "table",
	}

	specs := make([]Parameter, 0, len(params)+2)
	byLong := map[string]*Parameter{}
	byShort := map[string]*Parameter{}

	register := func(p Parameter) {
		if p.Name == "" {
			p.Name = p.flagName()
		}
		specs = append(specs, p)
		sp := &specs[len(specs)-1]
		long := sp.flagName()
		if long != "" {
			byLong[long] = sp
			if long != sp.Name {
				out.aliases[long] = sp.Name
			}
		}
		if s := sp.shortName(); s != "" {
			byShort[s] = sp
			if s != sp.Name {
				out.aliases[s] = sp.Name
			}
		}
	}

	for _, p := range params {
		register(p)
	}

	// Register global standard helpers unless overridden by caller params.
	builtinFormat := false
	if byLong["format"] == nil {
		builtinFormat = true
		register(Parameter{
			Name:    "format",
			Type:    "string",
			Default: "table",
			Enum:    []string{"json", "table"},
		})
	}
	builtinJSON := false
	if byLong["json"] == nil {
		builtinJSON = true
		register(Parameter{
			Name: "json",
			Type: "bool",
		})
	}

	setValue := func(sp *Parameter, raw string) error {
		if err := validateParamValue(sp, raw); err != nil {
			return err
		}
		if sp.isRepeatable() {
			out.values[sp.Name] = append(out.values[sp.Name], raw)
		} else {
			out.values[sp.Name] = []string{raw}
		}
		out.present[sp.Name] = true
		return nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out.positional = append(out.positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") && len(arg) > 2 {
			body := arg[2:]
			name, val, hasEq := strings.Cut(body, "=")
			sp := byLong[name]
			if sp == nil {
				return Parsed{}, fmt.Errorf("unknown flag: --%s", name)
			}
			if sp.paramType() == "bool" {
				if hasEq {
					if err := setValue(sp, val); err != nil {
						return Parsed{}, err
					}
				} else {
					if err := setValue(sp, "true"); err != nil {
						return Parsed{}, err
					}
				}
				continue
			}
			if !hasEq {
				if i+1 >= len(args) {
					return Parsed{}, fmt.Errorf("flag --%s requires a value", name)
				}
				i++
				val = args[i]
			}
			if err := setValue(sp, val); err != nil {
				return Parsed{}, err
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			body := arg[1:]
			name, val, hasEq := strings.Cut(body, "=")
			sp := byShort[name]
			if sp == nil {
				return Parsed{}, fmt.Errorf("unknown flag: -%s", name)
			}
			if sp.paramType() == "bool" {
				if hasEq {
					if err := setValue(sp, val); err != nil {
						return Parsed{}, err
					}
				} else {
					if err := setValue(sp, "true"); err != nil {
						return Parsed{}, err
					}
				}
				continue
			}
			if !hasEq {
				if i+1 >= len(args) {
					return Parsed{}, fmt.Errorf("flag -%s requires a value", name)
				}
				i++
				val = args[i]
			}
			if err := setValue(sp, val); err != nil {
				return Parsed{}, err
			}
			continue
		}
		out.positional = append(out.positional, arg)
	}

	// Apply defaults and check required flags.
	for i := range specs {
		sp := &specs[i]
		if !out.present[sp.Name] {
			if sp.Default != "" {
				if err := validateParamValue(sp, sp.Default); err != nil {
					return Parsed{}, err
				}
				out.values[sp.Name] = []string{sp.Default}
			} else if sp.Required {
				return Parsed{}, fmt.Errorf("missing required flag: --%s", sp.flagName())
			}
		}
	}

	// Resolve effective format.
	if out.present["format"] || builtinFormat {
		if f := out.String("format"); f != "" {
			out.format = f
		}
	}
	if (builtinJSON || out.present["json"]) && out.Bool("json") {
		out.format = "json"
	}

	return out, nil
}

func validateParamValue(sp *Parameter, raw string) error {
	flagLabel := "--" + sp.flagName()
	switch sp.paramType() {
	case "bool":
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("invalid boolean value %q for %s", raw, flagLabel)
		}
	case "int":
		if _, err := strconv.Atoi(raw); err != nil {
			return fmt.Errorf("invalid integer value %q for %s", raw, flagLabel)
		}
	case "float":
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return fmt.Errorf("invalid float value %q for %s", raw, flagLabel)
		}
	}
	if !sp.Free {
		if allowed := sp.allowedValues(); len(allowed) != 0 && !slices.Contains(allowed, raw) {
			return fmt.Errorf("invalid value %q for %s (allowed: %s)", raw, flagLabel, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// Parse parses c.Args against params using ParseFlags.
func (c *Ctx) Parse(params []Parameter) (Parsed, error) {
	return ParseFlags(c.Args, params)
}
