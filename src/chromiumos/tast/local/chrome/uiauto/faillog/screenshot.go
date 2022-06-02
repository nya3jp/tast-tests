// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides helper functions for dumping UI data on test failures.
package faillog

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Default screenshot file name.
const screenshotFileName = "screenshot.png"

// SaveScreenshotOnError takes screenshot when the test fails.
// Use SaveScreenshotToFileOnError, if you want to specify the fileName.
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
	if err := screenshot.CaptureChrome(ctx, cr, screenshotFile); err != nil {
		testing.ContextLog(ctx, "Failed to take screenshot: ", err)
	}
}
