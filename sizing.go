// ABOUTME: Aspect-ratio + pixel-size handling. Inferring from ref images,
// ABOUTME: parsing user --size input, mapping to per-family FAL conventions.

package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"
	"strings"
)

// Supported aspect ratios. Mirrors storyboard-gen's ASPECT_RATIO_MAP set.
// Anything else is snapped to the nearest of these.
var supportedAspectRatios = []string{"9:16", "1:1", "4:3", "16:9"}

// aspectRatioToImageSizePreset maps an aspect ratio to FAL's image_size
// preset string. Mirrors storyboard-gen ASPECT_RATIO_MAP.
var aspectRatioToImageSizePreset = map[string]string{
	"9:16": "portrait_16_9",
	"16:9": "landscape_16_9",
	"4:3":  "landscape_4_3",
	"1:1":  "square_hd",
}

// aspectRatioToPixelSize maps an aspect ratio to FAL's pixel "WxH" string.
// Mirrors storyboard-gen _PIXEL_SIZE_MAP. Note: 4:3 maps to 1536x1024 (which
// is actually 3:2) -- preserved verbatim from storyboard-gen for behaviour
// parity.
var aspectRatioToPixelSize = map[string]string{
	"9:16": "1024x1536",
	"16:9": "1536x1024",
	"4:3":  "1536x1024",
	"1:1":  "1024x1024",
}

// parseSizeFlag accepts either "W:H" (aspect ratio) or "WIDTHxHEIGHT" (pixel)
// and returns the canonical aspect-ratio form (snapped to a supported preset
// if necessary). An empty input returns "" (caller falls back).
func parseSizeFlag(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	// Pixel form: contains 'x' between two integers.
	if strings.Contains(strings.ToLower(input), "x") {
		w, h, err := splitInts(strings.ToLower(input), "x")
		if err != nil {
			return "", fmt.Errorf("invalid --size %q: expected WIDTHxHEIGHT (e.g. 1024x1024)", input)
		}
		if w <= 0 || h <= 0 {
			return "", fmt.Errorf("invalid --size %q: dimensions must be positive", input)
		}
		return snapToSupportedRatio(float64(w) / float64(h)), nil
	}

	// Aspect-ratio form: W:H.
	if strings.Contains(input, ":") {
		w, h, err := splitInts(input, ":")
		if err != nil {
			return "", fmt.Errorf("invalid --size %q: expected W:H (e.g. 16:9)", input)
		}
		if w <= 0 || h <= 0 {
			return "", fmt.Errorf("invalid --size %q: ratio components must be positive", input)
		}
		// If the input matches a supported ratio exactly, keep it; otherwise snap.
		canon := fmt.Sprintf("%d:%d", w, h)
		for _, r := range supportedAspectRatios {
			if r == canon {
				return canon, nil
			}
		}
		return snapToSupportedRatio(float64(w) / float64(h)), nil
	}

	return "", fmt.Errorf("invalid --size %q: expected W:H or WIDTHxHEIGHT", input)
}

func splitInts(s, sep string) (int, int, error) {
	parts := strings.SplitN(s, sep, 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected two integers separated by %q", sep)
	}
	a, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}

// inferAspectRatioFromRef opens the image at path, reads its dimensions
// (decoded from the header only -- no full pixel decode), and returns the
// nearest supported aspect ratio. On any error, returns "" (caller falls back).
func inferAspectRatioFromRef(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return ""
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ""
	}
	return snapToSupportedRatio(float64(cfg.Width) / float64(cfg.Height))
}

// snapToSupportedRatio picks the closest entry of supportedAspectRatios
// (compared on the W/H float value).
func snapToSupportedRatio(r float64) string {
	best := supportedAspectRatios[0]
	bestDiff := -1.0
	for _, ar := range supportedAspectRatios {
		w, h, _ := splitInts(ar, ":")
		diff := abs(float64(w)/float64(h) - r)
		if bestDiff < 0 || diff < bestDiff {
			best = ar
			bestDiff = diff
		}
	}
	return best
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// applySizing mutates payload to include the right sizing field for the
// handler's Sizing strategy. Empty aspectRatio or empty Sizing -> no-op.
func (h modelHandler) applySizing(payload map[string]interface{}, aspectRatio string) {
	if aspectRatio == "" || h.Sizing == "" {
		return
	}
	switch h.Sizing {
	case "image_size":
		if preset, ok := aspectRatioToImageSizePreset[aspectRatio]; ok {
			payload["image_size"] = preset
		}
	case "aspect_ratio":
		payload["aspect_ratio"] = aspectRatio
	case "pixel":
		if px, ok := aspectRatioToPixelSize[aspectRatio]; ok {
			payload["image_size"] = px
		}
	}
}

// outputFormatFromPath returns FAL's output_format value derived from the
// requested filename extension. Returns "" when the extension is missing or
// unrecognised -- caller skips the field, letting FAL choose its default.
func outputFormatFromPath(path string) string {
	dot := strings.LastIndex(path, ".")
	if dot < 0 || dot == len(path)-1 {
		return ""
	}
	switch strings.ToLower(path[dot+1:]) {
	case "png":
		return "png"
	case "jpg", "jpeg":
		return "jpeg"
	case "webp":
		return "webp"
	}
	return ""
}
