// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package annotations

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
)

// StartLogging clicks the "Start logging" button on the net export page.
func StartLogging(ctx context.Context, cr *chrome.Chrome, br *browser.Browser) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		errors.Wrap(err, "failed to create Test API connection")
	}

	netConn, err := br.NewConn(ctx, "chrome://net-export")
	if err != nil {
		errors.Wrap(err, "failed to load chrome://net-export")
	}

	// Click Start Log button.
	startLoggingBtn := `document.getElementById("start-logging")`
	if err := netConn.WaitForExpr(ctx, startLoggingBtn); err != nil {
		errors.Wrap(err, "failed to wait for the Start Logging button to load")

	}
	if err := netConn.Eval(ctx, startLoggingBtn+`.click()`, nil); err != nil {
		errors.Wrap(err, "failed to click the Start Logging button")
	}

	// Click Save button to choose the filename for log file.
	ui := uiauto.New(tconn)
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("Click 'Save' button",
		ui.WaitUntilExists(saveButton),
		ui.WaitUntilEnabled(saveButton),
		ui.DoDefault(saveButton),
		ui.WaitUntilGone(saveButton),
	)(ctx); err != nil {
		errors.Wrap(err, "failed to click")
	}
	return nil
}

// StopLoggingCheckLogs clicks the "Stop logging" button on the net export page and checks logs for given annotation.
func StopLoggingCheckLogs(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, annotation string) error {
	// Open the net-export page.
	netConn, err := br.NewConn(ctx, "chrome://net-export")
	if err != nil {
		errors.Wrap(err, "failed to load chrome://net-export")
	}

	// Click Stop Logging button.
	stopLoggingBtn := `document.getElementById("stop-logging")`
	if err := netConn.WaitForExpr(ctx, stopLoggingBtn); err != nil {
		errors.Wrap(err, "failed to wait for the Stop Logging button to load")
	}
	if err := netConn.Eval(ctx, stopLoggingBtn+`.click()`, nil); err != nil {
		errors.Wrap(err, "failed to click the Stop Logging button to load")
	}

	// Get the net export log file.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		errors.Wrap(err, "failed to get user's Download path")
	}
	downloadName := "chrome-net-export-log.json"
	downloadLocation := filepath.Join(downloadsPath, downloadName)

	// Read the net export log file.
	logFile, err := ioutil.ReadFile(downloadLocation)
	if err != nil {
		errors.Wrap(err, "failed to open logfile")
	}
	// Check if the traffic annotation exists in the log file.
	// Specifically checking for autofill_query:88863520.
	isExist, _ := regexp.Match(fmt.Sprintf("\"traffic_annotation\":%s", annotation), logFile)
	if !isExist {
		errors.Wrap(err, "failed to locate traffic annotation in logfile")
	}

	return nil
}
