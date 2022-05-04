// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package helpers contains helper functions for enterprise connector tests.
package helpers

import (
	"context"
	"net/http/httptest"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/testing"
)

// TestParams entail parameters describing the set policy for a user and more.
type TestParams struct {
	AllowsImmediateDelivery bool         // Specifies whether immediate delivery of files is allowed.
	AllowsUnscannableFiles  bool         // Specifies whether unscannable files (large or encrypted) are allowed.
	ScansEnabled            bool         // Specifies whether malware and dlp scans are enabled.
	BrowserType             browser.Type // Specifies the type of browser.
}

// TestFileParams describe the parameters for a test file.
type TestFileParams struct {
	TestName      string
	FileName      string
	UlBlockLabel  string
	IsBad         bool
	IsUnscannable bool
}

// ScanningTimeOut describes the typical time out for a scan.
const (
	ScanningTimeOut time.Duration = 2 * time.Minute
)

// GetTestFileParams returns the list of parameters for the files that should be tested.
func GetTestFileParams() []TestFileParams {
	return []TestFileParams{
		{
			TestName:      "Encrypted malware",
			FileName:      "unknown_malware_encrypted.zip",
			UlBlockLabel:  "encrypted",
			IsBad:         true,
			IsUnscannable: true,
		},
		{
			TestName:      "Unknown malware",
			FileName:      "unknown_malware.zip",
			UlBlockLabel:  "try again",
			IsBad:         true,
			IsUnscannable: false,
		},
		{
			TestName:      "Known malware",
			FileName:      "content.exe",
			UlBlockLabel:  "try again",
			IsBad:         true,
			IsUnscannable: false,
		},
		{
			TestName:      "DLP clear text",
			FileName:      "10ssns.txt",
			UlBlockLabel:  "try again",
			IsBad:         true,
			IsUnscannable: false,
		},
		{
			TestName:      "Allowed file",
			FileName:      "allowed.txt",
			UlBlockLabel:  "",
			IsBad:         false,
			IsUnscannable: false,
		},
	}
}

// WaitForDMTokenRegistered waits until a valid DM token exists.
// This is done by downloading unknown_malware.zip from `download.html`.
// This function fails if scanning is disabled.
func WaitForDMTokenRegistered(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, server *httptest.Server) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return checkDMTokenRegistered(ctx, br, tconn, server)
	}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for dm token to be registered")
	}
	return nil
}

func checkDMTokenRegistered(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, server *httptest.Server) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dconnSafebrowsing, err := br.NewConn(ctx, "chrome://safe-browsing/#tab-deep-scan")
	if err != nil {
		return testing.PollBreak(errors.Wrap(err, "failed to connect to chrome"))
	}
	defer dconnSafebrowsing.Close()
	defer dconnSafebrowsing.CloseTarget(cleanupCtx)

	dconn, err := br.NewConn(ctx, server.URL+"/download.html")
	defer dconn.Close()
	defer dconn.CloseTarget(cleanupCtx)

	// Close all prior notifications.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return testing.PollBreak(errors.Wrap(err, "failed to close notifications"))
	}

	err = dconn.Eval(ctx, `document.getElementById("unknown_malware.zip").click()`, nil)
	if err != nil {
		return testing.PollBreak(errors.Wrap(err, "failed to click on link to download file"))
	}

	// Check for notification (this might take some time in case of throttling).
	timeout := 2 * time.Minute

	_, err = ash.WaitForNotification(
		ctx,
		tconn,
		timeout,
		ash.WaitIDContains("notification-ui-manager"),
		ash.WaitMessageContains("unknown_malware.zip"),
	)
	if err != nil {
		return testing.PollBreak(errors.Wrap(err, "failed to wait for notification"))
	}

	// Remove file if it was downloaded.
	defer os.Remove(filesapp.DownloadPath + "unknown_malware.zip")

	var failedToGetToken bool
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		err := dconnSafebrowsing.Eval(ctx, `(async () => {
			const table = document.getElementById("deep-scan-list");
			if (table.rows.length == 0) {
				// If there is no entry, scanning is not yet complete.
				throw "Scanning is not yet complete";
			}
			// We check if the last entry is not empty to check whether there is an actual answer.
			innerHTML = table.rows[table.rows.length - 1].cells[1].innerHTML;
			if (innerHTML.includes("FAILED_TO_GET_TOKEN")) {
				return true;
			}
			if (innerHTML.includes("status")) {
				return false;
			}
			if (innerHTML.includes("UPLOAD_FAILURE")) {
				// Handle upload failure as token failure.
				return true;
			}
			throw "Scanning not yet complete";
			})()`, &failedToGetToken)
		if err != nil {
			testing.ContextLog(ctx, "Polling: ", err)
		}
		return err
	}, &testing.PollOptions{Timeout: timeout, Interval: 5 * time.Second}); err != nil {
		return testing.PollBreak(errors.Wrap(err, "failed to wait for dm token registration"))
	}

	if failedToGetToken {
		testing.ContextLog(ctx, "FAILED_TO_GET_TOKEN detected")
		return errors.New("FAILED_TO_GET_TOKEN detected")
	}

	return nil
}

// WaitForDeepScanningVerdict waits until a valid deep scanning verdict is found.
func WaitForDeepScanningVerdict(ctx context.Context, dconnSafebrowsing *browser.Conn, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		return dconnSafebrowsing.Eval(ctx, `(async () => {
			const table = document.getElementById("deep-scan-list");
			if (table.rows.length == 0) {
				// If there is no entry, scanning is not yet complete.
				throw "Scanning is not yet complete";
			}
			// We check if the last entry is not empty to check whether there is an actual answer.
			innerHTML = table.rows[table.rows.length - 1].cells[1].innerHTML;
			if (innerHTML.includes("status")) {
				return;
			}
			throw "Scanning not yet complete";
			})()`, nil)
	}, &testing.PollOptions{Timeout: timeout, Interval: 5 * time.Second})
}

// VerifyDeepScanningVerdict verifies that the deep scanning verdict corresponds to shouldBlock.
func VerifyDeepScanningVerdict(ctx context.Context, dconnSafebrowsing *browser.Conn, shouldBlock bool) error {
	var isBlocked bool
	err := dconnSafebrowsing.Eval(ctx, `(async () => {
		const table = document.getElementById("deep-scan-list");
		if (table.rows.length == 0) {
			// If there is no entry, scanning is not yet complete.
			throw 'No deep scanning verdict!';
		}
		if (table.rows[table.rows.length - 1].cells[1].innerHTML.length == 0) {
			throw 'Invalid empty response detected';
		}
		// We check if the last entry includes a block message.
		return table.rows[table.rows.length - 1].cells[1].innerHTML.includes("BLOCK");
		})()`, &isBlocked)
	if err != nil {
		var tableHTML string
		err2 := dconnSafebrowsing.Eval(ctx, `document.getElementById("deep-scan-list").outerHTML`, &tableHTML)
		if err2 != nil {
			return errors.Wrap(err2, "failed to get html of table")
		}
		return errors.Wrapf(err, "failed to check deep-scan-list entry. Html of table: %v", tableHTML)
	}
	if isBlocked != shouldBlock {
		var tableHTML string
		err := dconnSafebrowsing.Eval(ctx, `document.getElementById("deep-scan-list").outerHTML`, &tableHTML)
		if err != nil {
			return errors.Wrapf(err, "block state (%v) doesn't match expectation (%v). Failed to get html of table", isBlocked, shouldBlock)
		}
		return errors.Errorf("block state (%v) doesn't match expectation (%v). Html of table: %v", isBlocked, shouldBlock, tableHTML)
	}
	return nil
}

// GetSafeBrowsingExperimentEnabled checks whether the given safe browsing experiment is enabled.
func GetSafeBrowsingExperimentEnabled(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, experimentName string) (bool, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dconnSafebrowsing, err := br.NewConn(ctx, "chrome://safe-browsing/#preferences")
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer dconnSafebrowsing.Close()
	defer dconnSafebrowsing.CloseTarget(cleanupCtx)

	var experimentEnabled bool
	err = dconnSafebrowsing.Eval(ctx, `Array.from(document.getElementById("experiments-list").children).find((obj) => obj.innerHTML.includes("`+experimentName+`")).innerHTML.includes("Enabled:")`, &experimentEnabled)
	return experimentEnabled, err
}
