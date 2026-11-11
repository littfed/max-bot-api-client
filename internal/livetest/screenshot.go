package livetest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// CaptureHTMLScreenshot attempts to capture a PNG screenshot of the generated HTML report
// using an available system Chromium browser (Edge or Chrome).
func CaptureHTMLScreenshot(htmlPath, pngPath string) error {
	candidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		"msedge",
		"chrome",
		"google-chrome",
		"chromium",
	}

	var browserExe string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			browserExe = c
			break
		}
		if p, err := exec.LookPath(c); err == nil {
			browserExe = p
			break
		}
	}

	if browserExe == "" {
		return fmt.Errorf("no chromium browser (Edge/Chrome) found for screenshot capture")
	}

	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	absPNG, err := filepath.Abs(pngPath)
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(absPNG), 0755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, browserExe,
		"--headless",
		"--disable-gpu",
		"--hide-scrollbars",
		"--window-size=1200,900",
		fmt.Sprintf("--screenshot=%s", absPNG),
		"file:///"+filepath.ToSlash(absHTML),
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("browser screenshot command failed: %w (output: %s)", err, string(output))
	}

	if _, err := os.Stat(absPNG); err != nil {
		return fmt.Errorf("screenshot was not created on disk: %w", err)
	}

	return nil
}
