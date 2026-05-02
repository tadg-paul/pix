// ABOUTME: Regression tests for the generate-image CLI.
// ABOUTME: Each test runs the compiled binary as a subprocess, matching the real user entry point.

package regression

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
)

// buildBinary compiles the CLI binary once for all tests.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		tmpDir := os.TempDir()
		binaryPath = filepath.Join(tmpDir, "generate-image-test")
		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = filepath.Join(projectRoot())
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build failed: %s\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("Failed to build binary: %v", buildErr)
	}
	return binaryPath
}

// projectRoot returns the root of the project (two levels up from this test file).
func projectRoot() string {
	// tests/regression/ -> project root
	dir, _ := os.Getwd()
	return filepath.Join(dir, "..", "..")
}

// setupEnv creates a temp directory with .env and config.yaml next to a copy
// of the binary, so the binary resolves config from its own directory.
// A copy is used (not a symlink) because the binary resolves symlinks
// via filepath.EvalSymlinks -- matching production where make install copies.
func setupEnv(t *testing.T, binary string, falKey string, configYAML string) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Copy binary into tmpDir so it resolves config from there
	copyPath := filepath.Join(tmpDir, "generate-image")
	srcData, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("Failed to read binary: %v", err)
	}
	if err := os.WriteFile(copyPath, srcData, 0755); err != nil {
		t.Fatalf("Failed to copy binary: %v", err)
	}

	if falKey != "" {
		envContent := fmt.Sprintf("FAL_KEY=%s\n", falKey)
		if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(envContent), 0600); err != nil {
			t.Fatalf("Failed to write .env: %v", err)
		}
	}

	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}
	}

	return copyPath
}

// runBinary executes the binary with the given args, stdin, and env vars.
// Returns stdout, stderr, and the exit code.
func runBinary(t *testing.T, binPath string, args []string, stdin string, env []string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	} else {
		// Explicitly provide empty stdin (closed pipe, not terminal)
		cmd.Stdin = strings.NewReader("")
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), env...)

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("Failed to run binary: %v", err)
	}

	return stdout.String(), stderr.String(), exitCode
}

// startFakeAPI starts a test HTTP server that mimics the FAL API.
// generateHandler handles the image generation endpoint.
// pricingHandler handles the pricing endpoint (nil = 404).
// Returns the server and its URL.
func startFakeAPI(t *testing.T, generateHandler http.HandlerFunc, pricingHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// Catch-all for generation endpoints (any model path)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/models/pricing") {
			if pricingHandler != nil {
				pricingHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		if generateHandler != nil {
			generateHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// fakeImagePNG is a minimal valid PNG (1x1 red pixel).
var fakeImagePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
	0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// successHandler returns a handler that serves a fake image and records the request.
func successHandler(t *testing.T, imageServer *httptest.Server, capturedBody *string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", 500)
			return
		}
		if capturedBody != nil {
			*capturedBody = string(body)
		}
		resp := map[string]interface{}{
			"images": []map[string]interface{}{
				{"url": imageServer.URL + "/image.png"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Tests ---

// RT-1.1: No arguments exits non-zero with usage on stderr.
// User action: types "generate-image" with no arguments.
// User observes: error message on terminal, non-zero exit.
func TestCLI_no_args_exits_nonzero_with_usage_RT1_1(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	_, stderr, exitCode := runBinary(t, linkPath, []string{}, "", nil)

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	if !strings.Contains(strings.ToLower(stderr), "usage") {
		t.Errorf("Expected usage message on stderr, got: %q", stderr)
	}
}

// RT-1.2: Empty stdin exits non-zero with error on stderr.
// User action: runs "echo '' | generate-image out.png".
// User observes: error about empty prompt on terminal.
func TestCLI_empty_stdin_exits_nonzero_RT1_2(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	_, stderr, exitCode := runBinary(t, linkPath, []string{"out.png"}, "", nil)

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	if stderr == "" {
		t.Errorf("Expected error message on stderr, got nothing")
	}
}

// RT-1.3: Whitespace-only stdin exits non-zero with error on stderr.
// User action: runs "echo '   ' | generate-image out.png".
// User observes: error about empty prompt on terminal.
func TestCLI_whitespace_stdin_exits_nonzero_RT1_3(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	_, stderr, exitCode := runBinary(t, linkPath, []string{"out.png"}, "   \n\t  \n", nil)

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	if stderr == "" {
		t.Errorf("Expected error message on stderr, got nothing")
	}
}

// RT-1.4: Missing FAL_KEY exits non-zero with clear error.
// User action: runs the tool without a .env file.
// User observes: error about missing API key on terminal.
func TestCLI_missing_fal_key_exits_nonzero_RT1_4(t *testing.T) {
	bin := buildBinary(t)
	// No FAL_KEY in .env
	linkPath := setupEnv(t, bin, "", "model: fal-ai/grok-2-aurora\n")

	_, stderr, exitCode := runBinary(t, linkPath, []string{"out.png"}, "a red cat", nil)

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "fal_key") && !strings.Contains(lower, "api key") && !strings.Contains(lower, "fal key") {
		t.Errorf("Expected error mentioning FAL_KEY or API key, got: %q", stderr)
	}
}

// RT-1.5: Missing config.yaml exits non-zero with clear error.
// User action: runs the tool without a config.yaml file.
// User observes: error about missing config on terminal.
func TestCLI_missing_config_exits_nonzero_RT1_5(t *testing.T) {
	bin := buildBinary(t)
	// No config.yaml
	linkPath := setupEnv(t, bin, "test-key", "")

	_, stderr, exitCode := runBinary(t, linkPath, []string{"out.png"}, "a red cat", nil)

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "config") {
		t.Errorf("Expected error mentioning config, got: %q", stderr)
	}
}

// RT-1.6: FAL_KEY loaded from .env next to binary, not from cwd.
// User action: runs tool from /tmp where a decoy .env exists with a different key.
// User observes: the tool uses the key from its own directory, not /tmp.
func TestCLI_env_loaded_from_binary_dir_not_cwd_RT1_6(t *testing.T) {
	bin := buildBinary(t)

	// Set up the real env next to binary with a known key
	realKey := "real-key-from-binary-dir"
	binPath := setupEnv(t, bin, realKey, "model: fal-ai/grok-2-aurora\n")

	// Create a decoy .env in a different cwd with a wrong key
	cwd := t.TempDir()
	decoyKey := "decoy-key-from-cwd"
	os.WriteFile(filepath.Join(cwd, ".env"), []byte(fmt.Sprintf("FAL_KEY=%s\n", decoyKey)), 0600)

	// Start fake API that records the auth header
	var capturedAuth string
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	server := startFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		resp := map[string]interface{}{
			"images": []map[string]interface{}{
				{"url": imageServer.URL + "/image.png"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}, nil)

	outFile := filepath.Join(t.TempDir(), "out.png")

	cmd := exec.Command(binPath, outFile)
	cmd.Stdin = strings.NewReader("a red cat")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "FAL_BASE_URL="+server.URL)
	cmd.Run()

	expectedAuth := "Key " + realKey
	if capturedAuth != expectedAuth {
		t.Errorf("Expected auth header %q (from binary dir), got %q", expectedAuth, capturedAuth)
	}
}

// RT-1.7: Model read from config.yaml next to binary.
// User action: sets model to a specific value in config.yaml.
// User observes: that model is what the FAL API receives.
func TestCLI_model_from_config_yaml_RT1_7(t *testing.T) {
	bin := buildBinary(t)
	customModel := "fal-ai/test-model-xyz"
	linkPath := setupEnv(t, bin, "test-key", fmt.Sprintf("model: %s\n", customModel))

	var capturedPath string
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	server := startFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		resp := map[string]interface{}{
			"images": []map[string]interface{}{
				{"url": imageServer.URL + "/image.png"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}, nil)

	outFile := filepath.Join(t.TempDir(), "out.png")
	runBinary(t, linkPath, []string{outFile}, "a red cat", []string{"FAL_BASE_URL=" + server.URL})

	if !strings.Contains(capturedPath, customModel) {
		t.Errorf("Expected request path to contain model %q, got path: %q", customModel, capturedPath)
	}
}

// RT-1.8: Prompt text piped to stdin is passed to the FAL API.
// User action: pipes "a red cat sitting on a wall" to the tool.
// User observes: that exact prompt is sent to FAL.
func TestCLI_prompt_passed_to_api_RT1_8(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	var capturedBody string
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	server := startFakeAPI(t, successHandler(t, imageServer, &capturedBody), nil)

	prompt := "a red cat sitting on a wall"
	outFile := filepath.Join(t.TempDir(), "out.png")
	runBinary(t, linkPath, []string{outFile}, prompt, []string{"FAL_BASE_URL=" + server.URL})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(capturedBody), &parsed); err != nil {
		t.Fatalf("Failed to parse captured request body: %v", err)
	}
	if parsed["prompt"] != prompt {
		t.Errorf("Expected prompt %q in request body, got %q", prompt, parsed["prompt"])
	}
}

// RT-1.9: Successful generation writes a non-empty image file at the output path.
// User action: pipes a prompt and provides an output filename.
// User observes: an image file appears at that path.
func TestCLI_success_writes_image_file_RT1_9(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	server := startFakeAPI(t, successHandler(t, imageServer, nil), nil)

	outFile := filepath.Join(t.TempDir(), "out.png")
	_, _, exitCode := runBinary(t, linkPath, []string{outFile}, "a red cat", []string{"FAL_BASE_URL=" + server.URL})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("Output file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("Output file is empty")
	}
	// Verify it's a valid PNG (starts with PNG magic bytes)
	data, _ := os.ReadFile(outFile)
	if len(data) < 4 || string(data[:4]) != "\x89PNG" {
		t.Errorf("Output file does not appear to be a valid PNG")
	}
}

// RT-1.10: FAL API error exits non-zero with error on stderr.
// User action: FAL API returns an error (e.g. server error).
// User observes: error message on terminal, no file created.
func TestCLI_api_error_exits_nonzero_RT1_10(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	server := startFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail": "model not found"}`, http.StatusBadRequest)
	}, nil)

	outFile := filepath.Join(t.TempDir(), "out.png")
	_, stderr, exitCode := runBinary(t, linkPath, []string{outFile}, "a red cat", []string{"FAL_BASE_URL=" + server.URL})

	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code, got 0")
	}
	if stderr == "" {
		t.Errorf("Expected error message on stderr, got nothing")
	}
	// Output file should not exist
	if _, err := os.Stat(outFile); err == nil {
		t.Errorf("Output file should not exist after API error")
	}
}

// RT-1.11: --help flag prints usage and exits zero.
// User action: runs "generate-image --help".
// User observes: usage information printed, clean exit.
func TestCLI_help_flag_prints_usage_RT1_11(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	// --help should work without stdin
	_, stderr, exitCode := runBinary(t, linkPath, []string{"--help"}, "", nil)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --help, got %d", exitCode)
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "usage") {
		t.Errorf("Expected usage text for --help, got: %q", stderr)
	}
}

// RT-1.12: --version flag prints version and exits zero.
// User action: runs "generate-image --version".
// User observes: version string printed, clean exit.
func TestCLI_version_flag_prints_version_RT1_12(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	_, stderr, exitCode := runBinary(t, linkPath, []string{"--version"}, "", nil)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --version, got %d", exitCode)
	}
	if stderr == "" {
		t.Errorf("Expected version output, got nothing")
	}
}

// RT-1.13: Cost printed to stderr when pricing data is available.
// User action: generates an image with a model that has pricing data.
// User observes: cost line like "Cost: $0.07 (per image)" on terminal.
func TestCLI_cost_printed_when_available_RT1_13(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	pricingHandler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"prices": []map[string]interface{}{
				{
					"unit_price": 0.07,
					"unit":       "image",
					"currency":   "USD",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	server := startFakeAPI(t, successHandler(t, imageServer, nil), pricingHandler)

	outFile := filepath.Join(t.TempDir(), "out.png")
	_, stderr, _ := runBinary(t, linkPath, []string{outFile}, "a red cat", []string{"FAL_BASE_URL=" + server.URL})

	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "cost") {
		t.Errorf("Expected cost information on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "0.07") {
		t.Errorf("Expected cost amount on stderr, got: %q", stderr)
	}
}

// RT-1.14: No cost line on stderr when pricing data is unavailable.
// User action: generates an image with a model that has no pricing data.
// User observes: no cost line on terminal.
func TestCLI_no_cost_when_unavailable_RT1_14(t *testing.T) {
	bin := buildBinary(t)
	linkPath := setupEnv(t, bin, "test-key", "model: fal-ai/grok-2-aurora\n")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImagePNG)
	}))
	t.Cleanup(imageServer.Close)

	// Pricing endpoint returns empty prices
	pricingHandler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"prices": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	server := startFakeAPI(t, successHandler(t, imageServer, nil), pricingHandler)

	outFile := filepath.Join(t.TempDir(), "out.png")
	_, stderr, _ := runBinary(t, linkPath, []string{outFile}, "a red cat", []string{"FAL_BASE_URL=" + server.URL})

	lower := strings.ToLower(stderr)
	if strings.Contains(lower, "cost") {
		t.Errorf("Expected no cost line on stderr when pricing unavailable, got: %q", stderr)
	}
}
