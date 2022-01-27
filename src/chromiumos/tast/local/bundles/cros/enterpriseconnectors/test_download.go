// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/testing"
)

type userParams struct {
	username                string // username for gaia login.
	password                string // password for gaia login.
	allowsImmediateDelivery bool   // specifies whether immediate delivery of files is allowed
	allowsUnscannableFiles  bool   // specifies whether unscannable files (large or encrypted) are allowed
	scansEnabled            bool   // specifies whether malware and dlp scans are enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestDownload,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Enterprise connector test",
		Timeout:      20 * time.Minute,
		Contacts: []string{
			"sseckler@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		VarDeps: []string{
			"enterpriseconnectors.username1",
			"enterpriseconnectors.password1",
			"enterpriseconnectors.username2",
			"enterpriseconnectors.password2",
			"enterpriseconnectors.username3",
			"enterpriseconnectors.password3",
			"enterpriseconnectors.username4",
			"enterpriseconnectors.password4",
		},
		Params: []testing.Param{
			{
				Name: "scan_enabled_allows_immediate_and_unscannable",
				Val: userParams{
					username:                "enterpriseconnectors.username1",
					password:                "enterpriseconnectors.password1",
					allowsImmediateDelivery: true,
					allowsUnscannableFiles:  true,
					scansEnabled:            true,
				},
			},
			{
				Name: "scan_enabled_blocks_immediate_and_unscannable",
				Val: userParams{
					username:                "enterpriseconnectors.username2",
					password:                "enterpriseconnectors.password2",
					allowsImmediateDelivery: false,
					allowsUnscannableFiles:  false,
					scansEnabled:            true,
				},
			},
			{
				Name: "scan_disabled_allows_immediate_and_unscannable",
				Val: userParams{
					username:                "enterpriseconnectors.username3",
					password:                "enterpriseconnectors.password3",
					allowsImmediateDelivery: true,
					allowsUnscannableFiles:  true,
					scansEnabled:            false,
				},
			},
			{
				Name: "scan_disabled_blocks_immediate_and_unscannable",
				Val: userParams{
					username:                "enterpriseconnectors.username4",
					password:                "enterpriseconnectors.password4",
					allowsImmediateDelivery: false,
					allowsUnscannableFiles:  false,
					scansEnabled:            false,
				},
			},
		},
	})
}

func TestDownload(ctx context.Context, s *testing.State) {

	userParams := s.Param().(userParams)
	username := s.RequiredVar(userParams.username)
	password := s.RequiredVar(userParams.password)

	URL := "https://bce-testingsite.appspot.com/"

	// Start up Chrome.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ProdPolicy(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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

	// Create Browser
	br := cr.Browser()

	dconn, err := br.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer dconn.Close()

	// Needed to allow loading of enterprise connectors and reduce flakiness of tests.
	testing.Sleep(ctx, 20*time.Second)

	for _, param := range []struct {
		testName         string
		dlFileName       string
		dlLinkIdentifier string
		dlIsBad          bool
		dlIsUnscannable  bool
	}{
		{
			testName:         "Encrypted malware",
			dlFileName:       "unknown_malware_encrypted.zip",
			dlLinkIdentifier: "Encrypted file",
			dlIsBad:          true,
			dlIsUnscannable:  true,
		},
		{
			testName:         "Large file",
			dlFileName:       "100MB.bin",
			dlLinkIdentifier: "Large file",
			dlIsBad:          false,
			dlIsUnscannable:  true,
		},
		{
			testName:         "Unknown malware",
			dlFileName:       "unknown_malware.zip",
			dlLinkIdentifier: "Unknown malware",
			dlIsBad:          true,
			dlIsUnscannable:  false,
		},
		{
			testName:         "Known malware",
			dlFileName:       "content.exe",
			dlLinkIdentifier: "Known malware",
			dlIsBad:          true,
			dlIsUnscannable:  false,
		},
	} {
		s.Run(ctx, param.testName, func(ctx context.Context, s *testing.State) {
			dlFileName := param.dlFileName

			shouldBlockDownload := false
			if userParams.scansEnabled {
				if param.dlIsUnscannable {
					shouldBlockDownload = !userParams.allowsUnscannableFiles
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

			// err = dconn.Eval(ctx, `document.getElementById(`+param.dl_link_id+`).click()`, nil)
			err = dconn.Eval(ctx, `Array.from(document.getElementsByTagName("a")).filter(elem => elem.innerHTML.includes("`+param.dlLinkIdentifier+`"))[0].click()`, nil)
			if err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// Check notification
			timeout := 15 * time.Second
			if strings.Contains(param.testName, "Large") {
				s.Log("Increased timeout to 2 minutes because of large file")
				timeout = 2 * time.Minute
			}
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
			if err := files.WithTimeout(2 * time.Second).WaitForFile(dlFileName)(ctx); err != nil {
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
