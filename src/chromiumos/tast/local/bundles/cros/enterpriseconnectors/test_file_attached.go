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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/enterpriseconnectors/helpers"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestFileAttached,
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
			"download.html", // download.html required for CheckDMTokenRegistered.
			"file_input.html",
			"10ssns.txt",
			"allowed.txt",
			"content.exe",
			"unknown_malware_encrypted.zip",
			"unknown_malware.zip",
		},
	})
}

// TestFileAttached tests the correct behavior of the enterprise connectors when attaching a file to a webpage.
// Hereby, it is checked:
// 1. Whether a file is blocked or not
// 2. Whether the correct UI is shown
// 3. Whether the deep scan result is correct (especially relevant for AllowsImmediateDelivery==true)
func TestFileAttached(ctx context.Context, s *testing.State) {

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
	_, ok := devicePolicies.Chrome["OnFileAttachedEnterpriseConnector"]
	policyParams := s.Param().(helpers.PolicyParams)
	if !ok && policyParams.ScansEnabled {
		s.Fatal("Policy isn't set, but should be")
	}
	if ok && !policyParams.ScansEnabled {
		s.Fatal("Policy is set, but shouldn't be")
	}

	testUploadForBrowser(ctx, s, browser.TypeLacros)
	testUploadForBrowser(ctx, s, browser.TypeAsh)
}

func testUploadForBrowser(ctx context.Context, s *testing.State, browserType browser.Type) {

	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconn)

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
	defer dconn.CloseTarget(ctx)

	// Need to wait for a valid dm token, i.e., the proper initialization of the enterprise connectors
	s.Log("Checking for dm token")
	if err := testing.Poll(ctx, func(c context.Context) error {
		return helpers.CheckDMTokenRegistered(ctx, s, br, server)
	}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for dm token to be registered: ", err)
	}
	s.Log("Checking for dm token done")

	// Create test directory if it does not yet exist
	testDirPath := filepath.Join(filesapp.MyFilesPath, "test_dir")
	if _, err := os.Stat(testDirPath); os.IsNotExist(err) {
		s.Log("Create folder 'test'")
		if err := os.Mkdir(testDirPath, 0755); err != nil {
			s.Fatal("Failed to create test folder: ", err)
		}
		defer os.Remove(testDirPath)
	}

	for _, params := range helpers.GetTestFileParams() {
		s.Run(ctx, params.TestName, func(ctx context.Context, s *testing.State) {
			ulFileName := params.FileName
			policyParams := s.Param().(helpers.PolicyParams)
			shouldBlockUpload := false
			if policyParams.ScansEnabled {
				if params.IsUnscannable {
					shouldBlockUpload = !policyParams.AllowsUnscannableFiles
				} else {
					shouldBlockUpload = params.IsBad
				}
			}

			dconnSafebrowsing, err := br.NewConn(ctx, "chrome://safe-browsing/#tab-deep-scan")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconnSafebrowsing.Close()
			defer dconnSafebrowsing.CloseTarget(ctx)

			dconn, err := br.NewConn(ctx, server.URL+"/file_input.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconn.Close()
			defer dconn.CloseTarget(ctx)

			// Create file at test directory of MyFiles
			testFileLocation := filepath.Join(testDirPath, ulFileName)
			if _, err := os.Stat(testFileLocation); os.IsNotExist(err) {
				if err := fsutil.CopyFile(s.DataPath(ulFileName), testFileLocation); err != nil {
					s.Fatalf("Failed to copy the test video to %s: %v", testFileLocation, err)
				}
				defer os.Remove(testFileLocation)
			}

			// Click on <input type="file">
			fileInputNodeFinder := nodewith.Name("Choose File").Role(role.Button).First()
			err = ui.LeftClick(fileInputNodeFinder)(ctx)
			if err != nil {
				s.Fatal("Failed to press file input button: ", err)
			}

			// Create pseudo FilesApp. Should not be closed, as it's not actually an app, but just the file picker
			files, err := filesapp.App(ctx, tconn, filesapp.PickerPseudoAppID)
			if err != nil {
				s.Error("Failed to get window of picker: ", err)
			}

			// Open file in test_dir
			err = uiauto.Combine("open file",
				files.OpenDir("test_dir", "test_dir"),
				files.OpenFile(ulFileName),
			)(ctx)
			if err != nil {
				s.Error("Failed to open file: ", err)
			}
			err = ui.WaitUntilGone(nodewith.Name("Files").ClassName("WebContentsViewAura"))(ctx)

			scanningTimeOut := 2 * time.Minute

			// Check whether the scanning dialog is shown correctly
			if !policyParams.AllowsImmediateDelivery && policyParams.ScansEnabled {
				// Wait for scanning dialog to show and complete scanning
				scanningDialogFinder := nodewith.ClassName("DialogClientView")
				scanningLabelFinder := nodewith.Role(role.StaticText).ClassName("Label").NameStartingWith("Checking").Ancestor(scanningDialogFinder)
				err = uiauto.Combine("Scanning dialog",
					ui.WithTimeout(1*time.Second).WaitUntilExists(scanningLabelFinder),
					ui.WithTimeout(scanningTimeOut).WaitUntilGone(scanningLabelFinder),
				)(ctx)
				if err != nil {
					s.Error("Did not show scanning dialog: ", err)
				}

				if shouldBlockUpload {
					// Check that a blocked verdict is shown
					blockedLabelTextFinder := nodewith.Role(role.StaticText).ClassName("Label").Ancestor(scanningDialogFinder).NameContaining(params.UlBlockLabel)
					err = ui.WithTimeout(1 * time.Second).WaitUntilExists(blockedLabelTextFinder)(ctx)
					if err != nil {
						s.Error("Did not show scan blocked message: ", err)
					}

					// Close dialog
					closeButtonFinder := nodewith.Name("Close").Role(role.Button).Ancestor(scanningDialogFinder)
					err = ui.LeftClick(closeButtonFinder)(ctx)
					if err != nil {
						s.Error("Failed to close dialog: ", err)
					}
				} else {
					// Check that an allowed verdict is shown
					allowedLabelTextFinder := nodewith.Role(role.StaticText).ClassName("Label").Ancestor(scanningDialogFinder).NameContaining("file will be uploaded")
					err = ui.WithTimeout(1 * time.Second).WaitUntilExists(allowedLabelTextFinder)(ctx)
					if err != nil {
						s.Error("Did not show scan success message: ", err)
					}
					// For allowed, the dialog should be closed automatically
				}
				// Check that the dialog is closed
				err = ui.WithTimeout(5 * time.Second).WaitUntilGone(scanningDialogFinder)(ctx)
				if err != nil {
					s.Error("Did not close scanning dialog: ", err)
				}
			} else {
				// Check that no dialog will be opened
				scanningDialogFinder := nodewith.ClassName("DialogClientView")
				err = ui.EnsureGoneFor(scanningDialogFinder, 1*time.Second)(ctx)
				if err != nil {
					s.Error("Scanning dialog detected, but none was expected: ", err)
				}
			}

			// Ensure file was or was not attached, by checking javascript output
			var blocked bool
			err = dconn.Eval(ctx, `document.getElementsByTagName("input")[0].files.length == 0`, &blocked)
			if err != nil {
				s.Error("Failed to determine whether file was blocked: ", err)
			}
			if !policyParams.AllowsImmediateDelivery && shouldBlockUpload {
				if !blocked {
					s.Fatal("File should have been blocked, but wasn't")
				}
			} else {
				if blocked {
					s.Fatal("File shouldn't have been blocked, but was")
				}
			}

			if policyParams.ScansEnabled && !params.IsUnscannable {
				// If scans are enabled and the content isn't unscannable, we check the deep scanning verdict
				helpers.WaitForDeepScanningVerdict(ctx, s, dconnSafebrowsing, scanningTimeOut)
				helpers.VerifyDeepScanningVerdict(ctx, s, dconnSafebrowsing, params.IsBad)
			}
		})
	}
}
