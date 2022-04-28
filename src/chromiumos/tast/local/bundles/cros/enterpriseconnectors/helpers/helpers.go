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
	AllowsImmediateDelivery bool // Specifies whether immediate delivery of files is allowed.
	AllowsUnscannableFiles  bool // Specifies whether unscannable files (large or encrypted) are allowed.
	ScansEnabled            bool // Specifies whether malware and dlp scans are enabled.
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

	dconn, err := br.NewConn(ctx, server.URL+"/download.html")
	defer dconn.Close()

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
