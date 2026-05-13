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

	// Parse global flags up to the first non-flag (subcommand) or end of args.
	quiet := false
	helpRequested := false
	versionRequested := false
	subcommandIdx := -1

	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			subcommandIdx = i
			break
		}
		switch arg {
		case "-h", "--help":
			helpRequested = true
		case "--version":
			versionRequested = true
		case "-q", "--quiet":
			quiet = true
		default:
			// Non-global flag in the global position.
			fmt.Fprintf(os.Stderr, "Error: %s is not a global flag (must be placed after the subcommand)\n", arg)
			printUsage()
			return 2
		}
	}

	// --help is mutually exclusive with all other flags and arguments.
	if helpRequested {
		hasOther := versionRequested || quiet || subcommandIdx >= 0
		if hasOther {
			fmt.Fprintln(os.Stderr, "Error: --help cannot be combined with other flags or arguments")
			printUsage()
			return 2
		}
		printUsage()
		return 0
	}

	// --version takes precedence over subcommand dispatch (but not over --help).
	if versionRequested {
		if quiet || subcommandIdx >= 0 {
			fmt.Fprintln(os.Stderr, "Error: --version cannot be combined with other flags or arguments")
			printUsage()
			return 2
		}
		fmt.Println("pix " + version)
		return 0
	}

	// No subcommand provided after global flags.
	if subcommandIdx < 0 {
		printUsage()
		return 2
	}

	subcommand := args[subcommandIdx]
	subcommandArgs := args[subcommandIdx+1:]

	switch subcommand {
	case "generate", "gen":
		return runGenImg(subcommandArgs, quiet, subcommand)
	case "cost":
		return runCost(subcommandArgs, quiet)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		return 2
	}
}
