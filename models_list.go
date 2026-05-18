// ABOUTME: 'pix models' subcommand -- list FAL image-model endpoint_ids.
// ABOUTME: Plain-text output, one model per line, pipeable into grep/awk/etc.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func printModelsUsage() {
	printHelptext("models")
}

// runModelsList handles `pix models [substring]`. Lists active image-model
// endpoint_ids from FAL, one per line on stdout. Optional positional arg
// filters by regex (so it stays consistent with cost / preselect semantics).
func runModelsList(args []string, globalQuiet bool) int {
	helpRequested := false
	var filterArg string

	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			helpRequested = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
				printModelsUsage()
				return 2
			}
			if filterArg != "" {
				fmt.Fprintf(os.Stderr, "Error: only one filter argument is accepted (got %q and %q)\n", filterArg, arg)
				printModelsUsage()
				return 2
			}
			filterArg = arg
		}
	}

	if helpRequested {
		printModelsUsage()
		return 0
	}

	confDir, err := resolveConfDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	cfg, err := loadConfig(filepath.Join(confDir, "config.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	falKey, err := resolveFALKey(cfg, confDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	models, err := fetchAllImageModels(falKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	var re *regexp.Regexp
	if filterArg != "" {
		re, err = regexp.Compile(filterArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: filter %q is not a valid regex: %v\n", filterArg, err)
			return 2
		}
	}

	count := 0
	for _, m := range models {
		if re != nil && !re.MatchString(m.EndpointID) {
			continue
		}
		fmt.Println(m.EndpointID)
		count++
	}
	if count == 0 && filterArg != "" {
		fmt.Fprintf(os.Stderr, "(no image models match %q)\n", filterArg)
		return 1
	}
	return 0
}
