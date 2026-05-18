// ABOUTME: cost subcommand handler -- queries FAL pricing without generating an image.
// ABOUTME: Resolves model via positional arg / config substring / model picker.

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func printCostUsage() {
	printHelptext("cost")
}

// runCost handles the cost subcommand. Resolution order for which endpoint
// to query pricing on:
//
//  1. Positional argument (e.g. `pix cost xai/grok-imagine-image`): used verbatim.
//  2. cfg.Model resolves to a single match in the live /v1/models catalogue
//     (substring/regex against image-only models): use that match.
//  3. cfg.Model is ambiguous (zero matches OR multiple matches):
//     - TTY: fire the model picker, user selects.
//     - Non-TTY: error out.
//  4. cfg.Model is a complete endpoint_id (exact match in the catalogue or
//     contains a `/`): use as-is.
//
// This makes `pix cost` work whether the user has the full endpoint_id in
// config or a substring shorthand that the picker normally resolves.
func runCost(args []string, globalQuiet bool) int {
	dryRun := false
	var positionalModel string

	// -q/--quiet and -h/--help are consumed by main.go before reaching here.
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
				printCostUsage()
				return 2
			}
			if positionalModel != "" {
				fmt.Fprintf(os.Stderr, "Error: only one model argument is accepted (got %q and %q)\n", positionalModel, arg)
				printCostUsage()
				return 2
			}
			positionalModel = arg
		}
	}

	if globalQuiet {
		// Quiet mode: skip everything (no API call, no output).
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

	pricingBase := pricingBaseURL()

	if dryRun {
		// In dry-run we don't resolve substrings (we don't want to hit FAL
		// at all). Just show what would happen with the literal input.
		model := positionalModel
		if model == "" {
			model = cfg.Model
		}
		fmt.Fprintf(os.Stderr, "Model: %s\n", model)
		fmt.Fprintf(os.Stderr, "Would GET %s/v1/models/pricing?endpoint_id=%s\n", pricingBase, model)
		fmt.Fprintf(os.Stderr, "Would POST %s/v1/models/pricing/estimate (historical_api_price for %s)\n", pricingBase, model)
		fmt.Fprintln(os.Stderr, "(dry run -- no API calls made)")
		return 0
	}

	falKey, err := resolveFALKey(cfg, confDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Resolve which endpoint to look up pricing for.
	endpoint, err := resolveCostEndpoint(cfg, positionalModel, falKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Fprintf(os.Stderr, "Model: %s\n", endpoint)

	unitPrice, unit, err := fetchUnitPrice(client, pricingBase, endpoint, falKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unit price: not available (%v)\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Unit price: $%.2f per %s (source: FAL API)\n", unitPrice, unit)
	}

	estimate, err := fetchHistoricalEstimate(client, pricingBase, endpoint, falKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Estimated cost: not available (no usage history for this model)")
	} else {
		fmt.Fprintf(os.Stderr, "Estimated cost: $%.4f per call based on usage history (source: FAL API)\n", estimate)
	}

	return 0
}

// resolveCostEndpoint walks the resolution order documented on runCost.
func resolveCostEndpoint(cfg *config, positional, falKey string) (string, error) {
	// 1. Explicit positional wins.
	if positional != "" {
		return positional, nil
	}

	if cfg.Model == "" {
		return "", fmt.Errorf("no model: set 'model:' in config.yaml or pass a model id as an argument")
	}

	// 2. cfg.Model looks like a complete endpoint_id (contains '/') -- use
	//    it verbatim. Saves a round-trip and preserves pre-existing UX for
	//    users who configure full ids.
	if strings.Contains(cfg.Model, "/") {
		return cfg.Model, nil
	}

	// 3. Substring/regex resolution against the live catalogue.
	resolved, resErr := resolveModelBySubstring(falKey, cfg.Model)
	if resErr == nil {
		return resolved, nil
	}

	// 4. Ambiguous or no match.
	if isStdinTTY() {
		// TTY -- offer the picker. Reuse the model-picker flow, no refs.
		ep, cancelled, err := runModelPickerFlow(cfg, falKey, false)
		if err != nil {
			return "", fmt.Errorf("model picker failed: %v (after substring resolution: %v)", err, resErr)
		}
		if cancelled {
			return "", fmt.Errorf("model picker cancelled")
		}
		return ep, nil
	}

	// 5. Non-TTY and ambiguous -- propagate the resolver error.
	return "", resErr
}
