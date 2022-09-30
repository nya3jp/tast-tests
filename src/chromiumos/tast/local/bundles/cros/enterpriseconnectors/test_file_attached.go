// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/enterpriseconnectors/helpers"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filepicker"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestFileAttached,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Enterprise connector test for uploading files",
		Timeout:      20 * time.Minute,
		Contacts: []string{
			"sseckler@google.com",
			"cros-enterprise-connectors@google.com",
			"webprotect-eng@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Params: []testing.Param{
			{
				Name:    "scan_enabled_allows_immediate_and_unscannable_ash",
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledAllowExtra",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            true,
					BrowserType:             browser.TypeAsh,
				},
			},
			{
				Name:    "scan_enabled_blocks_immediate_and_unscannable_ash",
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledBlockExtra",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: false,
					AllowsUnscannableFiles:  false,
					ScansEnabled:            true,
					BrowserType:             browser.TypeAsh,
				},
			},
			{
				Name:    "scan_disabled_ash",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDisabled",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            false,
					BrowserType:             browser.TypeAsh,
				},
			},
			{
				Name:    "scan_enabled_allows_immediate_and_unscannable_lacros",
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledAllowExtra",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            true,
					BrowserType:             browser.TypeLacros,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "scan_enabled_blocks_immediate_and_unscannable_lacros",
				Fixture: "lacrosGaiaSignedInProdPolicyWPEnabledBlockExtra",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: false,
					AllowsUnscannableFiles:  false,
					ScansEnabled:            true,
					BrowserType:             browser.TypeLacros,
				},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:    "scan_disabled_lacros",
				Fixture: "lacrosGaiaSignedInProdPolicyWPDisabled",
				Val: helpers.TestParams{
					AllowsImmediateDelivery: true,
					AllowsUnscannableFiles:  true,
					ScansEnabled:            false,
					BrowserType:             browser.TypeLacros,
				},
				ExtraSoftwareDeps: []string{"lacros"},
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
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	// Clear Downloads directory.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	files, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		s.Fatal("Failed to get files from Downloads directory")
	}
	for _, file := range files {
		if err := os.RemoveAll(filepath.Join(downloadsPath, file.Name())); err != nil {
			s.Fatal("Failed to remove file: ", file.Name())
		}
	}

	// Verify policy.
	tconnAsh, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	devicePolicies, err := policyutil.PoliciesFromDUT(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Failed to get device policies: ", err)
	}
	_, ok := devicePolicies.Chrome["OnFileAttachedEnterpriseConnector"]
	testParams := s.Param().(helpers.TestParams)
	if !ok && testParams.ScansEnabled {
		s.Fatal("Policy isn't set, but should be")
	}
	if ok && !testParams.ScansEnabled {
		s.Fatal("Policy is set, but shouldn't be")
	}

	testFileAttachedForBrowser(ctx, s, testParams.BrowserType)
}

func testFileAttachedForBrowser(ctx context.Context, s *testing.State, browserType browser.Type) {
	testParams := s.Param().(helpers.TestParams)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconnAsh, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconnAsh)

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Ensure that there are no windows open.
	if err := ash.CloseAllWindows(ctx, tconnAsh); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}
	// Ensure that all windows are closed after test.
	defer ash.CloseAllWindows(cleanupCtx, tconnAsh)

	// Create Browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconnBrowser, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to browser's test API: ", err)
	}

	// The browsers sometimes restore some tabs, so we manually close all unneeded tabs.
	closeTabsFunc := browser.CloseAllTabs
	if testParams.BrowserType == browser.TypeLacros {
		// For lacros-Chrome, it should leave a new tab to keep the Chrome process alive.
		closeTabsFunc = browser.ReplaceAllTabsWithSingleNewTab
	}
	if err := closeTabsFunc(ctx, tconnBrowser); err != nil {
		s.Fatal("Failed to close all unneeded tabs: ", err)
	}
	defer closeTabsFunc(cleanupCtx, tconnBrowser)

	dconn, err := br.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer dconn.Close()
	defer dconn.CloseTarget(cleanupCtx)

	// Need to wait for a valid dm token, i.e., the proper initialization of the enterprise connectors.
	if testParams.ScansEnabled {
		s.Log("Checking for dm token")
		downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
		if err != nil {
			s.Fatal("Failed to get user's Download path: ", err)
		}
		if err := helpers.WaitForDMTokenRegistered(ctx, br, tconnAsh, server, downloadsPath); err != nil {
			s.Fatal("Failed to wait for DM token: ", err)
		}
	}

	// Create test directory if it does not yet exist.
	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's MyFiles path: ", err)
	}
	testDirPath := filepath.Join(myFilesPath, "test_dir")
	if _, err := os.Stat(testDirPath); os.IsNotExist(err) {
		if err := os.Mkdir(testDirPath, 0755); err != nil {
			s.Fatal("Failed to create test folder: ", err)
		}
		defer os.Remove(testDirPath)
	}

	for _, params := range helpers.GetTestFileParams() {
		if succeeded := s.Run(ctx, params.TestName, func(ctx context.Context, s *testing.State) {
			testFileAttachedForBrowserAndFile(ctx, params, testParams, br, s, server, testDirPath, ui, tconnAsh)
		}); !succeeded {
			// Stop, if the subtest fails as it might have left the state unusable.
			// It also prevents showing wrong errors on tastboard.
			break
		}
	}
}

func testFileAttachedForBrowserAndFile(
	ctx context.Context,
	params helpers.TestFileParams,
	testParams helpers.TestParams,
	br *browser.Browser,
	s *testing.State,
	server *httptest.Server,
	testDirPath string,
	ui *uiauto.Context,
	tconnAsh *chrome.TestConn,
) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ulFileName := params.FileName

	shouldBlockUpload := false
	if testParams.ScansEnabled {
		if params.IsUnscannable {
			shouldBlockUpload = !testParams.AllowsUnscannableFiles
		} else {
			shouldBlockUpload = params.IsBad
		}
	}
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	dconnSafebrowsing, err := helpers.GetCleanDconnSafebrowsing(ctx, cr, br)
	if err != nil {
		s.Fatal("Failed to get clean safe browsing page: ", err)
	}
	defer dconnSafebrowsing.Close()
	defer dconnSafebrowsing.CloseTarget(cleanupCtx)

	dconn, err := br.NewConn(ctx, server.URL+"/file_input.html")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer dconn.Close()
	defer dconn.CloseTarget(cleanupCtx)

	// Create file at test directory of MyFiles.
	testFileLocation := filepath.Join(testDirPath, ulFileName)
	if _, err := os.Stat(testFileLocation); os.IsNotExist(err) {
		if err := fsutil.CopyFile(s.DataPath(ulFileName), testFileLocation); err != nil {
			s.Fatalf("Failed to copy the file to %s: %v", testFileLocation, err)
		}
		defer os.Remove(testFileLocation)
	}

	// Click on <input type="file">.
	fileInputNodeFinder := nodewith.Name("Choose File").Role(role.Button).First()
	if err := ui.LeftClick(fileInputNodeFinder)(ctx); err != nil {
		s.Fatal("Failed to press file input button: ", err)
	}

	files, err := filepicker.Find(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Failed to get window of picker: ", err)
	}

	// Open file in test_dir.
	if err := uiauto.Combine("open file",
		files.OpenDir("test_dir"),
		// SelectFile is necessary for some ChromeOS architectures to properly open the file.
		files.WithTimeout(5*time.Second).SelectFile(ulFileName),
		files.WithTimeout(5*time.Second).OpenFile(ulFileName),
	)(ctx); err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	if err := ui.WithInterval(20 * time.Millisecond).WithTimeout(5 * time.Second).WaitUntilGone(nodewith.Name("Files").HasClass("WebContentsViewAura"))(ctx); err != nil {
		cr := s.FixtValue().(chrome.HasChrome).Chrome()
		path := filepath.Join(s.OutDir(), fmt.Sprintf("screenshot-failed-to-close-file-picker-%s.png", params.TestName))
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			s.Log("Failed to capture screenshot: ", err)
		}
		s.Fatal("Failed to wait for File picker to close: ", err)
	}

	verifyUIForFileAttached(ctx, shouldBlockUpload, params, testParams, br, s, server, testDirPath, ui)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Ensure file was or was not attached, by checking javascript output.
		var blocked bool
		if err := dconn.Eval(ctx, `document.getElementsByTagName("input")[0].files.length == 0`, &blocked); err != nil {
			s.Fatal("Failed to determine whether file was blocked: ", err)
		}
		if !testParams.AllowsImmediateDelivery && shouldBlockUpload {
			if !blocked {
				// If a file is attached even though it should have been blocked, this is an immediate error.
				return testing.PollBreak(errors.New("file should have been blocked, but wasn't"))
			}
		} else {
			if blocked {
				// Sometimes it takes time to attach a file, so we do polling here.
				return errors.New("file shouldn't have been blocked, but was")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 5 * time.Second}); err != nil {
		s.Fatal("Failed to verify whether file was correctly attached or blocked: ", err)
	}

	if testParams.ScansEnabled {
		// If scans are enabled and the content isn't unscannable, we check the deep scanning verdict.
		if err := helpers.WaitForDeepScanningVerdict(ctx, dconnSafebrowsing, helpers.ScanningTimeOut); err != nil {
			s.Fatal("Failed to wait for deep scanning verdict: ", err)
		}
		if !params.IsUnscannable {
			if err := helpers.VerifyDeepScanningVerdict(ctx, dconnSafebrowsing, params.IsBad); err != nil {
				s.Fatal("Failed to verify deep scanning verdict: ", err)
			}
		}
	}

}

func verifyUIForFileAttached(
	ctx context.Context,
	shouldBlockUpload bool,
	params helpers.TestFileParams,
	testParams helpers.TestParams,
	br *browser.Browser,
	s *testing.State,
	server *httptest.Server,
	testDirPath string,
	ui *uiauto.Context) {
	// Check whether the scanning dialog is shown correctly.
	if !testParams.AllowsImmediateDelivery && testParams.ScansEnabled {
		// Wait for scanning dialog to show and complete scanning.
		scanningDialogFinder := nodewith.HasClass("DialogClientView").First()
		scanningLabelFinder := nodewith.Role(role.StaticText).HasClass("Label").NameStartingWith("Checking").Ancestor(scanningDialogFinder)
		if err := uiauto.Combine("show scanning dialog",
			// 1. Wait until scanning started.
			ui.WithTimeout(2*time.Second).WithInterval(10*time.Millisecond).WaitUntilExists(scanningLabelFinder),
			// 2. Wait until scanning finished.
			ui.WithTimeout(helpers.ScanningTimeOut).WithInterval(10*time.Millisecond).WaitUntilGone(scanningLabelFinder),
		)(ctx); err != nil {
			s.Error("Did not show scanning dialog: ", err)
		}

		if shouldBlockUpload {
			// Check that a blocked verdict is shown.
			blockedLabelTextFinder := nodewith.Role(role.StaticText).HasClass("Label").Ancestor(scanningDialogFinder).NameContaining(params.UlBlockLabel)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(blockedLabelTextFinder)(ctx); err != nil {
				s.Error("Did not show scan blocked message: ", err)
			}

			// Close dialog.
			closeButtonFinder := nodewith.Name("Close").Role(role.Button).Ancestor(scanningDialogFinder)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(closeButtonFinder)(ctx); err != nil {
				s.Error("Did not show close button for blocked dialog: ", err)
			}
			if err := ui.LeftClick(closeButtonFinder)(ctx); err != nil {
				s.Error("Failed to close dialog: ", err)
			}
		} else {
			// Check that an allowed verdict is shown.
			allowedLabelTextFinder := nodewith.Role(role.StaticText).HasClass("Label").Ancestor(scanningDialogFinder).NameContaining("file will be uploaded")
			if err := ui.WithTimeout(5 * time.Second).WithInterval(10 * time.Millisecond).WaitUntilExists(allowedLabelTextFinder)(ctx); err != nil {
				s.Error("Did not show scan success message: ", err)
			}
			// For allowed, the dialog should be closed automatically.
		}
		// Check that the dialog is closed.
		if err := ui.WithTimeout(5 * time.Second).WaitUntilGone(scanningDialogFinder)(ctx); err != nil {
			s.Error("Did not close scanning dialog: ", err)
		}
	} else {
		// Check that no dialog will be opened.
		scanningDialogFinder := nodewith.HasClass("DialogClientView")
		if err := ui.EnsureGoneFor(scanningDialogFinder, 2*time.Second)(ctx); err != nil {
			s.Error("Scanning dialog detected, but none was expected: ", err)
		}
	}
}
