// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/sharesheet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sharesheet,
		Desc: "Install ARC app and share to app via Sharesheet",
		Contacts: []string{
			"benreich@chromium.org",
			"melzhang@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: []string{
			"arc.Sharesheet.username",
			"arc.Sharesheet.password",
		},
	})
}

func Sharesheet(ctx context.Context, s *testing.State) {
	const (
		appShareLabel        = "ARC Chrome Sharesheet Test"
		expectedFileName     = "test.txt"
		expectedFileContents = "test file contents"
		fileContentsID       = "org.chromium.arc.testapp.chromesharesheet:id/file_content"
	)

	username := s.RequiredVar("arc.Sharesheet.username")
	password := s.RequiredVar("arc.Sharesheet.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Setup the test file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, expectedFileName)
	if err := ioutil.WriteFile(testFileLocation, []byte(expectedFileContents), 0644); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup ARC device and UI Automator.
	arcDevice, uiAutomator, err := setUpARC(ctx, cr, s.OutDir())
	if err != nil {
		s.Fatal("Failed setting up ARC: ", err)
	}
	defer arcDevice.Close()
	defer uiAutomator.Close(ctx)

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeSharesheetTest.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed launching the Files app: ", err)
	}
	defer files.Release(ctx)

	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed navigating to Downloads directory: ", err)
	}

	if err := files.SelectFile(ctx, expectedFileName); err != nil {
		s.Fatalf("Failed selecting file %q: %v", expectedFileName, err)
	}

	if err := files.ClickShareButton(ctx); err != nil {
		s.Fatal("Failed clicking share button: ", err)
	}

	if err := sharesheet.ClickApp(ctx, tconn, appShareLabel); err != nil {
		s.Fatal("Failed clicking app on stable sharesheet: ", err)
	}

	// Wait for the file contents to show in the Android test app.
	fileContentField := uiAutomator.Object(ui.ID(fileContentsID), ui.Text(expectedFileContents))
	if err := fileContentField.WaitForExists(ctx, 15*time.Second); err != nil {
		s.Fatalf("Failed waiting for file contents %q to appear in ARC window: %v", expectedFileContents, err)
	}
}

// setUpARC starts an ARC device and starts UI automator.
func setUpARC(ctx context.Context, cr *chrome.Chrome, outDir string) (*arc.ARC, *arcui.Device, error) {
	// Setup ARC device.
	arcDevice, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start ARC")
	}

	// Start up UI automator.
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(); err != nil {
			testing.ContextLog(ctx, "Failed closing UI automator: ", err)
		}
		return nil, nil, errors.Wrap(err, "failed initializing UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed waiting for intent helper")
	}

	return arcDevice, uiAutomator, nil
}
