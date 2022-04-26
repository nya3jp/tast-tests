// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/enterpriseconnectors/helpers"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// policyParams entail parameters describing the set policy for a user.
type policyParams struct {
	AllowsImmediateDelivery bool // specifies whether immediate delivery of files is allowed
	AllowsUnscannableFiles  bool // specifies whether unscannable files (large or encrypted) are allowed
	ScansEnabledForDownload bool // specifies whether malware and dlp scans are enabled for download
	ScansEnabledForUpload   bool // specifies whether malware and dlp scans are enabled for upload
}

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
				Val: policyParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabledForDownload: true,
					ScansEnabledForUpload:   false,
				},
			},
			{
				Name:    "scan_enabled_blocks_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDownloadBlockExtra",
				Val: policyParams{
					AllowsImmediateDelivery: false,
					AllowsUnscannableFiles:  false,
					ScansEnabledForDownload: true,
					ScansEnabledForUpload:   false,
				},
			},
			{
				Name:    "scan_disabled_allows_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPUploadAllowExtra",
				Val: policyParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabledForDownload: false,
					ScansEnabledForUpload:   true,
				},
			},
			{
				Name:    "scan_disabled_blocks_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPUploadBlockExtra",
				Val: policyParams{
					AllowsImmediateDelivery: false,
					AllowsUnscannableFiles:  false,
					ScansEnabledForDownload: false,
					ScansEnabledForUpload:   true,
				},
			},
		},
		Data: []string{
			"download.html",
			"10ssns.txt",
			"content.exe",
			"unknown_malware_encrypted.zip",
			"unknown_malware.zip",
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
	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	devicePolicies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Could not get device policies: ", err)
	}
	_, ok := devicePolicies.Chrome["OnFileDownloadedEnterpriseConnector"]
	policyParams := s.Param().(policyParams)
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
	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

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

	// Need to wait for a valid dm token, i.e., the proper initialization of the enterprise connectors
	s.Log("Checking for dm token")
	if err := testing.Poll(ctx, func(c context.Context) error {
		return helpers.CheckDMTokenRegistered(ctx, s, br)
	}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for dm token to be registered: ", err)
	}
	s.Log("Checking for dm token done")

	for _, param := range []struct {
		testName        string
		dlFileName      string
		dlIsBad         bool
		dlIsUnscannable bool
	}{
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
			policyParams := s.Param().(policyParams)
			shouldBlockDownload := false
			if policyParams.ScansEnabledForDownload {
				if param.dlIsUnscannable {
					shouldBlockDownload = !policyParams.AllowsUnscannableFiles
				} else {
					shouldBlockDownload = param.dlIsBad
				}
			}

			dconn, err := br.NewConn(ctx, server.URL+"/download.html")
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

			// Cleanup file
			defer func() {
				_, err = os.Stat(filesapp.DownloadPath + dlFileName)
				if !os.IsNotExist(err) {
					if err := os.Remove(filesapp.DownloadPath + dlFileName); err != nil {
						s.Error("Failed to remove ", dlFileName, ": ", err)
					}
				}
			}()

			// Check for notification (this might take some time in case of throttling)
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
			_, err = os.Stat(filesapp.DownloadPath + dlFileName)
			if os.IsNotExist(err) {
				if !shouldBlockDownload {
					s.Error("Download was blocked, but shouldn't have been: ", err)
				}
			} else {
				if shouldBlockDownload {
					s.Error("Download was not blocked")
				}
			}
		})
	}
}
