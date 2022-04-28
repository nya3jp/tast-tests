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

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestDownload,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Enterprise connector test for downloading files",
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
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledAllowExtra",
				Val: helpers.PolicyParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            true,
				},
			},
			{
				Name:    "scan_enabled_blocks_immediate_and_unscannable",
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledBlockExtra",
				Val: helpers.PolicyParams{
					AllowsImmediateDelivery: false,
					AllowsUnscannableFiles:  false,
					ScansEnabled:            true,
				},
			},
			{
				Name:    "scan_disabled",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDisabled",
				Val: helpers.PolicyParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            false,
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
	policyParams := s.Param().(helpers.PolicyParams)
	if !ok && policyParams.ScansEnabled {
		s.Fatal("Policy isn't set, but should be")
	}
	if ok && !policyParams.ScansEnabled {
		s.Fatal("Policy is set, but shouldn't be")
	}

	testDownloadForBrowser(ctx, s, browser.TypeLacros)
	testDownloadForBrowser(ctx, s, browser.TypeAsh)
}

func testDownloadForBrowser(ctx context.Context, s *testing.State, browserType browser.Type) {
	policyParams := s.Param().(helpers.PolicyParams)

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
		return helpers.CheckDMTokenRegistered(ctx, s, br, server)
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
			shouldBlockDownload := false
			if policyParams.ScansEnabled {
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
