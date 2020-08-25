// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides a post-test hook to save "faillog" on test failures.
// A faillog is a collection of log files which can be used to debug test failures.
package faillog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Save saves a faillog unconditionally.
func Save(ctx context.Context) {
	// If test setup failed, then the output dir may not exist.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}
	SaveToDir(ctx, dir)
}

// SaveToDir saves fail log to a specific directory
func SaveToDir(ctx context.Context, dir string) {
	if _, err := os.Stat(dir); err != nil {
		return
	}

	dir = filepath.Join(dir, "faillog")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	saveProcessList(dir)
	saveUpstartJobs(ctx, dir)
	saveScreenshot(ctx, dir)
}

// saveProcessList saves "ps" output.
func saveProcessList(dir string) {
	path := filepath.Join(dir, "ps.txt")
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	cmd := exec.Command("ps", "auxwwfZ")
	cmd.Stdout = f
	cmd.Run()
}

// saveScreenshot saves a screenshot.
func saveScreenshot(ctx context.Context, dir string) {
	if err := saveScreenshotCDP(ctx, dir); err != nil {
		testing.ContextLog(ctx, "Failed to take screenshot by Chrome API: ", err)
	} else {
		return
	}

	// Fallback to native screenshot command.
	if err := saveScreenshotNative(ctx, dir); err != nil {
		testing.ContextLog(ctx, "Failed to take screenshot by a command: ", err)
	}
}

// saveScreenshotNative saves a screenshot by using "screenshot" command.
func saveScreenshotNative(ctx context.Context, dir string) error {
	path := filepath.Join(dir, "screenshot_native.png")
	return screenshot.Capture(ctx, path)
}

// saveScreenshotCDP saves a screenshot by using Chrome API.
func saveScreenshotCDP(ctx context.Context, dir string) error {
	sm, err := cdputil.NewSession(ctx, cdputil.DebuggingPortPath)
	if err != nil {
		return errors.Wrap(err, "failed to create a new Chrome Devtools Protocol session")
	}

	bgURL := chrome.ExtensionBackgroundPageURL(chrome.TestExtensionID)
	all, err := sm.FindTargets(ctx, chrome.MatchTargetURL(bgURL))
	if len(all) == 0 {
		// Target not found.
		return errors.New("the background page of the test extension not found")
	}

	co, err := sm.NewConn(ctx, all[0].TargetID)
	if err != nil {
		return errors.Wrap(err, "failed to make a new Conn")
	}
	path := filepath.Join(dir, "screenshot_chrome.png")
	testing.ContextLog(ctx, "Taking screenshot via chrome API")
	return screenshot.CaptureCDP(ctx, co, path)
}

func saveUpstartJobs(ctx context.Context, dir string) {
	path := filepath.Join(dir, "initctl_list.log")
	if err := upstart.DumpJobs(ctx, path); err != nil {
		testing.ContextLog(ctx, "Failed to take a snapshot of all upstart jobs' status: ", err)
	}
}
