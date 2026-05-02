// ABOUTME: CLI tool that generates images from text prompts via the FAL API.
// ABOUTME: Reads prompt from stdin, writes image to the path given as $1.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const version = "0.1.0"

type config struct {
	Model string `yaml:"model"`
}

type falResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
}

type pricingResponse struct {
	Prices []struct {
		UnitPrice float64 `json:"unit_price"`
		Unit      string  `json:"unit"`
		Currency  string  `json:"currency"`
	} `json:"prices"`
}

func main() {
	os.Exit(run())
}

func run() int {
	// Check flags first -- before any other validation.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help":
			fmt.Fprintln(os.Stderr, "Usage: generate-image <output-file>")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Reads a text prompt from stdin and generates an image via the FAL API.")
			fmt.Fprintln(os.Stderr, "The output file path is required as the first argument.")
			return 0
		case "--version":
			fmt.Fprintln(os.Stderr, "generate-image "+version)
			return 0
		}
	}

	// Validate arguments.
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: generate-image <output-file>")
		return 2
	}
	outputPath := os.Args[1]

	// Read prompt from stdin.
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		return 1
	}
	prompt := strings.TrimSpace(string(stdinBytes))
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "Error: no prompt provided on stdin")
		return 1
	}

	// Resolve binary directory for config files.
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving executable path: %v\n", err)
		return 1
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving symlinks: %v\n", err)
		return 1
	}
	binDir := filepath.Dir(exePath)
	confDir := configDir(binDir)

	// Load FAL_KEY from .env.
	falKey, err := loadFALKey(filepath.Join(confDir, ".env"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Load config.
	cfg, err := loadConfig(filepath.Join(confDir, "config.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Determine API base URL (test hook via env var).
	baseURL := os.Getenv("FAL_BASE_URL")
	if baseURL == "" {
		baseURL = "https://queue.fal.run"
	}

	// Call FAL API.
	client := &http.Client{Timeout: 120 * time.Second}

	imageData, err := generateImage(client, baseURL, cfg.Model, prompt, falKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Write image to output path.
	if err := os.WriteFile(outputPath, imageData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		return 1
	}

	// Fetch and report pricing.
	reportCost(client, baseURL, cfg.Model, falKey)

	return 0
}

// configDir returns the directory containing .env and config.yaml.
// It checks the binary directory first (development), then falls back
// to ~/.config/generate-image/ (installed via make install).
func configDir(binDir string) string {
	if _, err := os.Stat(filepath.Join(binDir, ".env")); err == nil {
		return binDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return binDir
	}
	candidate := filepath.Join(home, ".config", "generate-image")
	if _, err := os.Stat(filepath.Join(candidate, ".env")); err == nil {
		return candidate
	}
	// Fall back to binDir so error messages reference the expected location.
	return binDir
}

// loadFALKey reads FAL_KEY from a .env file.
func loadFALKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read .env: %w (expected FAL_KEY in %s)", err, path)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(key) == "FAL_KEY" {
			v := strings.TrimSpace(value)
			if v != "" {
				return v, nil
			}
		}
	}

	return "", fmt.Errorf("FAL_KEY not found in %s", path)
}

// loadConfig reads config.yaml.
func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config.yaml: %w (expected config at %s)", err, path)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config.yaml: %w", err)
	}

	if cfg.Model == "" {
		return nil, fmt.Errorf("config.yaml: 'model' field is required")
	}

	return &cfg, nil
}

// generateImage calls the FAL API and returns the image bytes.
func generateImage(client *http.Client, baseURL, model, prompt, falKey string) ([]byte, error) {
	reqBody, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", baseURL, model)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+falKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FAL API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read FAL API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FAL API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var falResp falResponse
	if err := json.Unmarshal(body, &falResp); err != nil {
		return nil, fmt.Errorf("failed to parse FAL API response: %w", err)
	}

	if len(falResp.Images) == 0 {
		return nil, fmt.Errorf("FAL API returned no images")
	}

	// Download the image.
	imgResp, err := client.Get(falResp.Images[0].URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer imgResp.Body.Close()

	imageData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return imageData, nil
}

// reportCost fetches pricing from the FAL API and prints cost to stderr.
func reportCost(client *http.Client, baseURL, model, falKey string) {
	// Determine pricing URL -- use FAL_BASE_URL if set (for tests),
	// otherwise use the real pricing endpoint.
	pricingBase := baseURL
	if pricingBase == "https://queue.fal.run" {
		pricingBase = "https://api.fal.ai"
	}

	url := fmt.Sprintf("%s/v1/models/pricing?endpoint_id=%s", pricingBase, model)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Key "+falKey)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var pricing pricingResponse
	if err := json.Unmarshal(body, &pricing); err != nil {
		return
	}

	if len(pricing.Prices) == 0 {
		return
	}

	price := pricing.Prices[0]
	fmt.Fprintf(os.Stderr, "Cost: $%.2f (per %s)\n", price.UnitPrice, price.Unit)
}
