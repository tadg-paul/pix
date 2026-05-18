// ABOUTME: Model picker flow for --pick-model.
// ABOUTME: Fetches FAL /v1/models, presents to the configured picker, returns the selected endpoint_id.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type modelEntry struct {
	EndpointID string `json:"endpoint_id"`
	Metadata   struct {
		DisplayName string   `json:"display_name"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		Status      string   `json:"status"`
		Tags        []string `json:"tags"`
		ModelURL    string   `json:"model_url"`
		LicenseType string   `json:"license_type"`
		UpdatedAt   string   `json:"updated_at"`
	} `json:"metadata"`
}

type modelsListResponse struct {
	Models []modelEntry `json:"models"`
}

// runModelPickerFlow fetches FAL models for the given category, presents them
// via the configured picker, and returns the selected endpoint_id.
// hasRefs determines category: text-to-image with zero refs, image-to-image with refs.
func runModelPickerFlow(cfg *config, falKey string, hasRefs bool) (string, bool, error) {
	picker := effectivePicker(cfg)

	fields := strings.Fields(picker)
	if len(fields) == 0 {
		return "", false, fmt.Errorf("picker is empty")
	}
	pickerBin := fields[0]
	expandedBin, err := expandTilde(pickerBin)
	if err != nil {
		return "", false, err
	}
	if _, err := exec.LookPath(expandedBin); err != nil {
		return "", false, fmt.Errorf("picker %q not found on PATH: %w", pickerBin, err)
	}

	category := "text-to-image"
	if hasRefs {
		category = "image-to-image"
	}

	models, err := fetchModels(falKey, category)
	if err != nil {
		return "", false, err
	}
	if len(models) == 0 {
		return "", false, fmt.Errorf("FAL /v1/models returned no models for category=%s", category)
	}

	// model-picker.filter is a regex applied at the pix layer BEFORE fzf sees
	// the candidate list. Non-matching models are dropped. Invalid regex warns
	// and proceeds unfiltered.
	if filter := cfg.Interactive.ModelPicker.Filter; filter != "" {
		re, rerr := regexp.Compile(filter)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "Warning: model-picker.filter %q is not a valid regex: %v (proceeding without filter)\n", filter, rerr)
		} else {
			kept := make([]modelEntry, 0, len(models))
			for _, m := range models {
				if re.MatchString(m.EndpointID) {
					kept = append(kept, m)
				}
			}
			if len(kept) == 0 {
				return "", false, fmt.Errorf("no models match filter %q for category=%s", filter, category)
			}
			models = kept
		}
	}

	// Each candidate line is just the endpoint_id -- a clean list, easy to scan
	// and search. Model metadata is written to per-model files in a tempdir, so
	// fzf's preview pane can show prettified details for whichever line the user
	// is currently highlighting (preview command is `cat <tempdir>/{}.md`).
	tempDir, err := os.MkdirTemp("", "pix-model-info-")
	if err != nil {
		return "", false, fmt.Errorf("creating model-info tempdir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Reorder so the preselected endpoint_id (if present) is the first
	// candidate. fzf highlights the first line by default, giving the user a
	// one-keystroke confirm. If preselect is empty or doesn't match any model,
	// the candidate order is the default (API order).
	models = reorderPreselect(models, cfg.Interactive.ModelPicker.Preselect)

	candidates := make([]string, 0, len(models))
	for _, m := range models {
		candidates = append(candidates, m.EndpointID)
		if err := writeModelDetails(tempDir, m); err != nil {
			return "", false, fmt.Errorf("writing model details: %w", err)
		}
	}

	headerArg := "--header='Select a FAL model (" + category + ")'"
	previewArg := "--preview='cat " + tempDir + "/{}.md'"
	fzfArgs := []string{
		headerArg,
		previewArg,
		"--preview-window=right:60%:wrap",
	}
	selected, cancelled, err := invokePicker(picker, candidates, fzfArgs...)
	if err != nil {
		return "", false, err
	}
	if cancelled {
		return "", true, nil
	}

	endpointID := strings.TrimSpace(selected)
	if endpointID == "" {
		return "", true, nil
	}
	return endpointID, false, nil
}

// writeModelDetails writes a prettified markdown file describing the model
// at <tempDir>/<endpoint_id>.md, creating intermediate directories as needed
// (endpoint_id contains slashes, e.g. fal-ai/flux/dev).
func writeModelDetails(tempDir string, m modelEntry) error {
	path := filepath.Join(tempDir, m.EndpointID+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var sb strings.Builder
	if m.Metadata.DisplayName != "" {
		sb.WriteString(m.Metadata.DisplayName + "\n")
		sb.WriteString(strings.Repeat("-", len(m.Metadata.DisplayName)) + "\n\n")
	}
	sb.WriteString("ID:       " + m.EndpointID + "\n")
	if m.Metadata.Category != "" {
		sb.WriteString("Category: " + m.Metadata.Category + "\n")
	}
	if len(m.Metadata.Tags) > 0 {
		sb.WriteString("Tags:     " + strings.Join(m.Metadata.Tags, ", ") + "\n")
	}
	if m.Metadata.LicenseType != "" {
		sb.WriteString("Licence:  " + m.Metadata.LicenseType + "\n")
	}
	if m.Metadata.UpdatedAt != "" {
		sb.WriteString("Updated:  " + m.Metadata.UpdatedAt + "\n")
	}
	if m.Metadata.Description != "" {
		sb.WriteString("\n" + m.Metadata.Description + "\n")
	}
	// Plain URL -- iTerm2 / Terminal.app auto-linkify, no OSC 8 needed.
	if m.Metadata.ModelURL != "" {
		sb.WriteString("\nDocs: " + m.Metadata.ModelURL + "\n")
	} else {
		sb.WriteString("\nDocs: https://fal.ai/models/" + m.EndpointID + "\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// reorderPreselect moves the first entry whose EndpointID is matched by the
// preselect regex to the front of the slice. If preselect is empty, no entry
// matches, or the regex fails to compile, the slice is returned unchanged
// (with a stderr warning on compile failure). Stable for the remaining entries.
func reorderPreselect(models []modelEntry, preselect string) []modelEntry {
	if preselect == "" {
		return models
	}
	re, err := regexp.Compile(preselect)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: model-picker.preselect %q is not a valid regex: %v (proceeding without preselect)\n", preselect, err)
		return models
	}
	for i, m := range models {
		if re.MatchString(m.EndpointID) {
			if i == 0 {
				return models
			}
			out := make([]modelEntry, 0, len(models))
			out = append(out, m)
			out = append(out, models[:i]...)
			out = append(out, models[i+1:]...)
			return out
		}
	}
	return models
}

// fetchAllImageModels fetches both text-to-image and image-to-image catalogues
// in parallel, dedupes by endpoint_id (text-to-image wins on collision), and
// returns the merged list sorted by endpoint_id.
//
// Used by `pix cost` (substring resolution) and `pix models` (listing). The
// model picker for gen/edit uses fetchModels() with a single category derived
// from refs presence.
func fetchAllImageModels(falKey string) ([]modelEntry, error) {
	type result struct {
		models []modelEntry
		err    error
	}
	tti := make(chan result, 1)
	iti := make(chan result, 1)
	go func() {
		ms, e := fetchModels(falKey, "text-to-image")
		tti <- result{ms, e}
	}()
	go func() {
		ms, e := fetchModels(falKey, "image-to-image")
		iti <- result{ms, e}
	}()
	r1, r2 := <-tti, <-iti
	if r1.err != nil && r2.err != nil {
		return nil, fmt.Errorf("fetching /v1/models: text-to-image: %v; image-to-image: %v", r1.err, r2.err)
	}

	seen := make(map[string]bool, len(r1.models)+len(r2.models))
	merged := make([]modelEntry, 0, len(r1.models)+len(r2.models))
	for _, m := range r1.models {
		if !seen[m.EndpointID] {
			seen[m.EndpointID] = true
			merged = append(merged, m)
		}
	}
	for _, m := range r2.models {
		if !seen[m.EndpointID] {
			seen[m.EndpointID] = true
			merged = append(merged, m)
		}
	}
	// Sort alphabetically for predictable output and substring matching order.
	for i := 1; i < len(merged); i++ {
		for j := i; j > 0 && merged[j-1].EndpointID > merged[j].EndpointID; j-- {
			merged[j-1], merged[j] = merged[j], merged[j-1]
		}
	}
	return merged, nil
}

// resolveModelBySubstring matches a substring/regex against the live FAL model
// catalogue (both image categories). Returns the matched endpoint_id when
// exactly one model matches. Empty input or zero/multiple matches return ""
// along with a descriptive error.
//
// Uses Go's regexp.Compile so users who want anchored or alternation matches
// can write them; bare substrings work too since they compile as literal
// regex patterns.
func resolveModelBySubstring(falKey, pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("no model id or substring provided")
	}
	models, err := fetchAllImageModels(falKey)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("model substring %q is not a valid regex: %w", pattern, err)
	}
	var matches []string
	for _, m := range models {
		if re.MatchString(m.EndpointID) {
			matches = append(matches, m.EndpointID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no image model matches %q", pattern)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous: %d models match %q (e.g. %s)", len(matches), pattern, matches[0])
	}
}

// fetchModels queries FAL's /v1/models endpoint for active models in the given category.
func fetchModels(falKey, category string) ([]modelEntry, error) {
	params := url.Values{}
	params.Set("category", category)
	params.Set("status", "active")
	params.Set("limit", "100")

	endpoint := fmt.Sprintf("%s/v1/models?%s", pricingBaseURL(), params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building /v1/models request: %w", err)
	}
	if falKey != "" {
		req.Header.Set("Authorization", "Key "+falKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching /v1/models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /v1/models response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FAL /v1/models returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed modelsListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing /v1/models response: %w", err)
	}
	return parsed.Models, nil
}
