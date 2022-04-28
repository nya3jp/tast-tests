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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/testing"
)

// PolicyParams entail parameters describing the set policy for a user.
type PolicyParams struct {
	AllowsImmediateDelivery bool // specifies whether immediate delivery of files is allowed
	AllowsUnscannableFiles  bool // specifies whether unscannable files (large or encrypted) are allowed
	ScansEnabled            bool // specifies whether malware and dlp scans are enabled
}

type testFileParams struct {
	TestName      string
	FileName      string
	UlBlockLabel  string
	IsBad         bool
	IsUnscannable bool
}

// GetTestFileParams returns the list of parameters for the files that should be tested.
func GetTestFileParams() []testFileParams {
	return []testFileParams{
		{
			TestName:      "Encrypted malware",
			FileName:      "unknown_malware_encrypted.zip",
			UlBlockLabel:  "ncrypted",
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

// CheckDMTokenRegistered waits until a valid DM token exists.
// This is done by downloading unknown_malware_encrypted.zip from `download.html`.
func CheckDMTokenRegistered(ctx context.Context, s *testing.State, br *browser.Browser, server *httptest.Server) error {
	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Close all prior notifications
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	dconnSafebrowsing, err := br.NewConn(ctx, "chrome://safe-browsing/#tab-deep-scan")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer dconnSafebrowsing.Close()
	defer dconnSafebrowsing.CloseTarget(ctx)

	dconn, err := br.NewConn(ctx, server.URL+"/download.html")
	defer dconn.Close()
	defer dconn.CloseTarget(ctx)

	err = dconn.Eval(ctx, `document.getElementById("unknown_malware_encrypted.zip").click()`, nil)

	// Check for notification (this might take some time in case of throttling)
	timeout := 2 * time.Minute

	deadline, _ := ctx.Deadline()
	s.Log("Context deadline is ", deadline)
	ntfctn, err := ash.WaitForNotification(
		ctx,
		tconn,
		timeout,
		ash.WaitIDContains("notification-ui-manager"),
		ash.WaitMessageContains("unknown_malware_encrypted.zip"),
	)
	if err != nil {
		s.Fatalf("Failed to wait for notification with title %q: %v", "", err)
	}

	// Remove file if it was downloaded
	if ntfctn.Title == "Download complete" {
		if err := os.Remove(filesapp.DownloadPath + "unknown_malware_encrypted.zip"); err != nil {
			s.Error("Failed to remove ", "unknown_malware_encrypted.zip", ": ", err)
		}
	}

	// Check safebrowsing page, whether there wasn't a failed_to_get_token error in the last message
	var failedToGetToken bool
	err = dconnSafebrowsing.Eval(ctx, `(async () => {
		table = document.getElementById("deep-scan-list");
		if (table.rows.length == 0) {
			// If there is no entry, scanning is disabled and the token doesn't matter
			return false;
		}
		// Otherwise we check if the last entry includes a missing token
		return table.rows[table.rows.length - 1].cells[1].innerHTML.includes("FAILED_TO_GET_TOKEN");
		})()`, &failedToGetToken)
	if err != nil {
		s.Fatal("Failed to check deep-scan-list entry: ", err)
	}
	if failedToGetToken {
		s.Log("FAILED_TO_GET_TOKEN detected")
		return errors.New("FAILED_TO_GET_TOKEN detected")
	}

	return nil
}

// WaitForDeepScanningVerdict waits until a valid deep scanning verdict is found.
func WaitForDeepScanningVerdict(ctx context.Context, s *testing.State, dconnSafebrowsing *browser.Conn, timeout time.Duration) {
	if err := testing.Poll(ctx, func(c context.Context) error {
		var scanningComplete bool
		err := dconnSafebrowsing.Eval(ctx, `(async () => {
			table = document.getElementById("deep-scan-list");
			if (table.rows.length == 0) {
				// If there is no entry, scanning is not yet complete
				return false;
			}
			// Otherwise we check if the last entry includes a status field to check whether there is an actual answer
			return table.rows[table.rows.length - 1].cells[1].innerHTML.includes("status");
			})()`, &scanningComplete)
		if err != nil {
			s.Fatal("Failed to check deep-scan-list entry: ", err)
		}
		if !scanningComplete {
			s.Log("Scanning not yet complete")
			return errors.New("Scanning not yet complete")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for deep scanning verdict: ", err)
	}
}

// VerifyDeepScanningVerdict verifies that the deep scanning verdict corresponds to shouldBlock.
func VerifyDeepScanningVerdict(ctx context.Context, s *testing.State, dconnSafebrowsing *browser.Conn, shouldBlock bool) {
	var isBlocked bool
	err := dconnSafebrowsing.Eval(ctx, `(async () => {
		table = document.getElementById("deep-scan-list");
		if (table.rows.length == 0) {
			// If there is no entry, scanning is not yet complete
			throw 'No deep scanning verdict!';
		}
		// Otherwise we check if the last entry includes a status field to check whether there is an actual answer
		return table.rows[table.rows.length - 1].cells[1].innerHTML.includes("BLOCK");
		})()`, &isBlocked)
	if err != nil {
		s.Fatal("Failed to check deep-scan-list entry: ", err)
	}
	if isBlocked != shouldBlock {
		s.Error("Block state (", isBlocked, ") doesn't match expectation (", shouldBlock, ")")
	}
}
