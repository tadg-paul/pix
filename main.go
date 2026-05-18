// ABOUTME: pix CLI entry point.
// ABOUTME: Parses global flags, dispatches to subcommand handlers (generate, cost).

package main

import (
	"fmt"
	"os"
	"strings"
)

const version = "0.3.0"

func main() {
	os.Exit(run())
}

func printUsage() {
	printHelptext("pix")
}

func run() int {
	args := os.Args[1:]

	// No args: print usage and exit zero (help-first convention).
	if len(args) == 0 {
		printUsage()
		return 0
	}

	// Single-pass classification. Recognised global flags (-q/--quiet, --version)
	// are consumed regardless of position. -h/--help is top-level when it appears
	// before any subcommand token, subcommand-level otherwise. Every other flag
	// (and any non-flag token after the first) goes into subcommandArgs for the
	// subcommand handler to interpret.
	quiet := false
	helpTopLevel := false
	versionRequested := false
	subcommand := ""
	var subcommandArgs []string

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			if subcommand == "" {
				subcommand = arg
			} else {
				subcommandArgs = append(subcommandArgs, arg)
			}
			continue
		}
		switch arg {
		case "-q", "--quiet":
			quiet = true
		case "--version":
			versionRequested = true
		case "-h", "--help":
			if subcommand == "" {
				helpTopLevel = true
			} else {
				subcommandArgs = append(subcommandArgs, arg)
			}
		default:
			subcommandArgs = append(subcommandArgs, arg)
		}
	}

	// Determine subcommand-level --help and split subcommandArgs accordingly.
	helpSubcommand := false
	var nonHelpSubArgs []string
	for _, a := range subcommandArgs {
		if a == "-h" || a == "--help" {
			helpSubcommand = true
			continue
		}
		nonHelpSubArgs = append(nonHelpSubArgs, a)
	}

	// Top-level --help: mutually exclusive with every other flag/arg.
	if helpTopLevel {
		if versionRequested || quiet || subcommand != "" || len(subcommandArgs) > 0 {
			fmt.Fprintln(os.Stderr, "Error: --help cannot be combined with other flags or arguments")
			printUsage()
			return 2
		}
		printUsage()
		return 0
	}

	// Subcommand-level --help: mutually exclusive with every other flag/arg.
	if helpSubcommand {
		if versionRequested || quiet || len(nonHelpSubArgs) > 0 {
			fmt.Fprintln(os.Stderr, "Error: --help cannot be combined with other flags or arguments")
			// Print the subcommand's helptext on error (rather than top-level)
			// since the user clearly intended a subcommand invocation.
			switch subcommand {
			case "generate", "gen":
				printHelptext("generate")
			case "cost":
				printHelptext("cost")
			default:
				printUsage()
			}
			return 2
		}
		switch subcommand {
		case "generate", "gen":
			printHelptext("generate")
		case "cost":
			printHelptext("cost")
		case "models":
			printHelptext("models")
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
			printUsage()
			return 2
		}
		return 0
	}

	// --version: mutually exclusive with every other flag/arg.
	if versionRequested {
		if quiet || subcommand != "" || len(subcommandArgs) > 0 {
			fmt.Fprintln(os.Stderr, "Error: --version cannot be combined with other flags or arguments")
			printUsage()
			return 2
		}
		fmt.Println("pix " + version)
		return 0
	}

	// No subcommand. If we still have flags pending, they're unknown at the top level.
	if subcommand == "" {
		if len(subcommandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", subcommandArgs[0])
			printUsage()
			return 2
		}
		printUsage()
		return 2
	}

	switch subcommand {
	case "generate", "gen":
		return runGenImg(subcommandArgs, quiet, subcommand)
	case "cost":
		return runCost(subcommandArgs, quiet)
	case "models":
		return runModelsList(subcommandArgs, quiet)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		return 2
	}
}
