// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides helper functions for dumping UI data on test failures.
package faillog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Default screenshot file name.
const screenshotFileName = "screenshot.png"

// SaveScreenshotOnError takes screenshot when the test fails.
// Use SaveScreenshotToFileOnError if you want to specify the fileName.
func SaveScreenshotOnError(ctx context.Context, cr *chrome.Chrome, outDir string, hasError func() bool) {
	SaveScreenshotToFileOnError(ctx, cr, outDir, hasError, screenshotFileName)
}

// SaveScreenshotToFileOnError checks the given hasError function and takes screenshot into a file 'fileName' when the test fails.
// It does nothing when the test succeeds.
func SaveScreenshotToFileOnError(ctx context.Context, cr *chrome.Chrome, outDir string, hasError func() bool, fileName string) {
	if !hasError() {
		return
	}

	dir := filepath.Join(outDir, faillogDir)
	if err := os.MkdirAll(dir, 0777); err != nil {
		testing.ContextLogf(ctx, "Failed to create directory %s: %v", dir, err)
		return
	}

	screenshotFile := filepath.Join(dir, fileName)
	testing.ContextLog(ctx, "Test failed. Saving screenshot to ", screenshotFile)
	// Take screenshot for internal display.
	if err := screenshot.CaptureChrome(ctx, cr, screenshotFile); err != nil {
		testing.ContextLog(ctx, "Failed to take screenshot: ", err)
		// Return on error whithout further trying external displays.
		return
	}

	// Get display info and take screenshots for external displays.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to obtain test API conn to check the existence of external displays: ", err)
		return
	}
	displayInfos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get display info: ", err)
		return
	}
	baseName := strings.TrimSuffix(fileName, ".png")
	for _, info := range displayInfos {
		if info.IsInternal {
			continue
		}
		fileNameDisplay := fmt.Sprintf("%s-display-%s.png", baseName, info.ID)
		screenshotFileDisplay := filepath.Join(dir, fileNameDisplay)
		testing.ContextLogf(ctx, "Saving screenshot of display %q to %s", info.ID, screenshotFileDisplay)
		if err := screenshot.CaptureChromeForDisplay(ctx, cr, info.ID, screenshotFileDisplay); err != nil {
			testing.ContextLogf(ctx, "Failed to capture screenshot for display %q: %v", info.ID, err)
		}
	}
}
