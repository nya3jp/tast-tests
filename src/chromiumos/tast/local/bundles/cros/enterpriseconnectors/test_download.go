// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/enterpriseconnectors/fixtvals"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {

	testing.AddTest(&testing.Test{
		Func:         TestDownload,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Enterprise connector test",
		Timeout:      10 * time.Minute,
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"lacros",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Params: []testing.Param{
			{
				Name:    "scan_enabled_allows_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDownloadAllowExtra",
			},
			{
				Name:    "scan_enabled_blocks_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDownloadBlockExtra",
			},
			{
				Name:    "scan_disabled_allows_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPUploadAllowExtra",
			},
			{
				Name:    "scan_disabled_blocks_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPUploadBlockExtra",
			},
		},
	})
}

func TestDownload(ctx context.Context, s *testing.State) {

	// Clear Downloads directory.
	files, err := ioutil.ReadDir(filesapp.DownloadPath)
	if err != nil {
		s.Fatal("Failed to get files from Downloads directory")
	}
	for _, file := range files {
		if err = os.RemoveAll(filepath.Join(filesapp.DownloadPath, file.Name())); err != nil {
			s.Fatal("Failed to remove file: ", file.Name())
		}
	}

	// Verify policy
	tconn := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn()
	devicePolicies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Could not get device policies: ", err)
	}
	_, ok := devicePolicies.Chrome["OnFileDownloadedEnterpriseConnector"]
	policyParams := s.FixtValue().(*fixtvals.FixtValue).PolicyParams
	if !ok && policyParams.ScansEnabledForDownload {
		s.Fatal("Policy isn't set, but should be")
	}
	if ok && !policyParams.ScansEnabledForDownload {
		s.Fatal("Policy is set, but shouldn't be")
	}

	testDownloadForBrowser(ctx, s, browser.TypeLacros)
	testDownloadForBrowser(ctx, s, browser.TypeAsh)
}

func testDownloadForBrowser(ctx context.Context, s *testing.State, browserType browser.Type) {
	URL := "https://bce-testingsite.appspot.com/"

	tconn := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Create Browser
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	dconn, err := br.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer dconn.Close()

	for _, param := range []struct {
		testName        string
		dlFileName      string
		dlIsBad         bool
		dlIsUnscannable bool
	}{
		{
			testName:        "Large file",
			dlFileName:      "100MB.bin",
			dlIsBad:         false,
			dlIsUnscannable: true,
		},
		{
			testName:        "Encrypted malware",
			dlFileName:      "unknown_malware_encrypted.zip",
			dlIsBad:         true,
			dlIsUnscannable: true,
		},
		{
			testName:        "Unknown malware",
			dlFileName:      "unknown_malware.zip",
			dlIsBad:         true,
			dlIsUnscannable: false,
		},
		{
			testName:        "Known malware",
			dlFileName:      "content.exe",
			dlIsBad:         true,
			dlIsUnscannable: false,
		},
		{
			testName:        "DLP clear text",
			dlFileName:      "10ssns.txt",
			dlIsBad:         true,
			dlIsUnscannable: false,
		},
	} {
		s.Run(ctx, param.testName, func(ctx context.Context, s *testing.State) {
			dlFileName := param.dlFileName
			policyParams := s.FixtValue().(*fixtvals.FixtValue).PolicyParams
			shouldBlockDownload := false
			if policyParams.ScansEnabledForDownload {
				if param.dlIsUnscannable {
					shouldBlockDownload = !policyParams.AllowsUnscannableFiles
				} else {
					shouldBlockDownload = param.dlIsBad
				}
			}

			dconn, err := br.NewConn(ctx, URL)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconn.Close()

			// Close all prior notifications.
			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			// The file name is also the ID of the link elements.
			err = dconn.Eval(ctx, `document.getElementById('`+param.dlFileName+`').click()`, nil)
			if err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// Check for notification (this might take some time for large files or in case of throttling)
			timeout := 2 * time.Minute

			deadline, _ := ctx.Deadline()
			s.Log("Context deadline is ", deadline)
			ntfctn, err := ash.WaitForNotification(
				ctx,
				tconn,
				timeout,
				ash.WaitIDContains("notification-ui-manager"),
				ash.WaitMessageContains(dlFileName),
			)
			if err != nil {
				s.Fatalf("Failed to wait for notification with title %q: %v", "", err)
			}

			if shouldBlockDownload {
				if ntfctn.Title != "Dangerous download blocked" {
					s.Fatal("Download should be blocked, but wasn't. Notification: ", ntfctn)
				}
			} else {
				if ntfctn.Title != "Download complete" {
					s.Fatal("Download should be allowed, but wasn't. Notification: ", ntfctn)
				}
			}

			// Check file blocked/existence
			files, err := filesapp.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Launching the Files App failed: ", err)
			}
			defer files.Close(ctx)
			if err := files.OpenDownloads()(ctx); err != nil {
				s.Fatal("Opening Downloads folder failed: ", err)
			}
			if err := files.WithTimeout(5 * time.Second).WaitForFile(dlFileName)(ctx); err != nil {
				if !shouldBlockDownload {
					if errors.Is(err, context.DeadlineExceeded) {
						s.Error("Download was blocked: ", err)
					} else {
						s.Fatal("Failed to wait for ", dlFileName, ": ", err)
					}
				}
			} else {
				if shouldBlockDownload {
					s.Error("Download was not blocked")
				}
				if err := os.Remove(filesapp.DownloadPath + dlFileName); err != nil {
					s.Error("Failed to remove ", dlFileName, ": ", err)
				}
			}
		})
	}
}
