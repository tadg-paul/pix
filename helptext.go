// ABOUTME: Embedded help text for each pix subcommand.
// ABOUTME: Sources live in docs/helptext/*.md and are vetted alongside project docs.

package main

import (
	"embed"
	"fmt"
	"os"
)

//go:embed docs/helptext/pix.md docs/helptext/generate.md docs/helptext/cost.md
var helptextFS embed.FS

// printHelptext writes the named help-text file to stderr. The name is the
// subcommand identifier (e.g. "pix", "generate", "cost").
func printHelptext(name string) {
	data, err := helptextFS.ReadFile("docs/helptext/" + name + ".md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "help text for %q not available\n", name)
		return
	}
	os.Stderr.Write(data)
}
