package oden

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNamespaceAliasDispatchesBeforeDiscoveredExternal(t *testing.T) {
	for _, inlineOrder := range []string{"none", "before", "after"} {
		t.Run(inlineOrder, func(t *testing.T) {
			dir := t.TempDir()
			legacy := filepath.Join(dir, "mtm-lab")
			legacyHit := filepath.Join(dir, "legacy-executed")
			inlineHit := filepath.Join(dir, "inline-executed")
			input := filepath.Join(dir, "input with spaces.json")
			output := filepath.Join(dir, "output with spaces.json")
			if err := os.WriteFile(legacy, []byte("#!/bin/sh\nprintf legacy > \"$LEGACY_HIT\"\nexit 17\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(input, []byte("{\"revision\":2}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			// Inject discovery so neither PATH nor the discovery cache touches
			// the developer's environment. The losing binary is a real fake
			// executable, so accidentally dispatching it has an observable effect.
			fsys := fakeFS{dir: {"mtm-lab": 0755}}
			env := []string{"HOME=" + dir, "XDG_CONFIG_HOME=" + dir, "XDG_STATE_HOME=" + dir, "XDG_DATA_HOME=" + dir, "LEGACY_HIT=" + legacyHit}
			app, _, _ := testApp(t, fsys, dir, WithEnv(env))
			own := Command{Name: "lab", Run: func(context.Context, []string) error {
				return os.WriteFile(inlineHit, []byte("inline-owned"), 0600)
			}}
			if inlineOrder == "before" {
				app.Add(own)
			}
			app.Add(Command{Name: "lab.", Aliases: []string{"lab"}, Namespace: "lab", NamespaceHead: true,
				Children: []Command{{Name: "record", Children: []Command{{Name: "export",
					Parameters: []Parameter{{Name: "path", Type: "path", Required: true}, {Name: "output", Type: "path", Required: true}},
					Run: func(_ context.Context, args []string) error {
						fs := flag.NewFlagSet("export", flag.ContinueOnError)
						fs.SetOutput(io.Discard)
						destination := fs.String("output", "", "destination file")
						if err := fs.Parse(args); err != nil {
							return err
						}
						data, err := os.ReadFile(fs.Arg(0))
						if err != nil {
							return err
						}
						return os.WriteFile(*destination, data, 0600)
					},
				}}}},
			})
			if inlineOrder == "after" {
				app.Add(own)
			}
			entry := app.Catalog().Lookup("lab")
			if entry == nil || entry.Kind != KindInline {
				t.Fatalf("namespace word is not inline: %+v", entry)
			}
			if len(entry.Shadowed) != 1 && inlineOrder == "none" {
				t.Fatalf("namespace lost its external claim: %+v", entry)
			}
			if inlineOrder == "none" && (entry.Shadowed[0].Source != legacy || entry.Shadowed[0].Merged) {
				t.Fatalf("namespace must shadow, never merge, the old binary: %+v", entry.Shadowed)
			}
			app.Args = []string{"lab", "record", "export", "--output", output, input}
			if err := app.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if inlineOrder != "none" {
				data, err := os.ReadFile(inlineHit)
				if err != nil || string(data) != "inline-owned" {
					t.Fatalf("exact inline command lost precedence: %q %v", data, err)
				}
				// The explicit namespace remains available beside the app's
				// exact command, with the same registered export handler.
				app.Args[0] = "lab."
				if err := app.Run(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(output)
			if err != nil || string(data) != "{\"revision\":2}\n" {
				t.Fatalf("namespace export did not preserve path arguments: %q %v", data, err)
			}
			if _, err := os.Stat(legacyHit); !os.IsNotExist(err) {
				t.Fatalf("discovered old binary ran: %v", err)
			}
		})
	}
}
