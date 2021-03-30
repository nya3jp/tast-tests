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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		SoftwareDeps: []string{"chrome"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			// TODO(b/179510073): Reenable when the test is passing.
			// ExtraAttr:         []string{"group:mainline", "informational"},
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

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
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	sr, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to initialize the screen recorder: ", err)
	}

	if err := sr.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the screen recorder: ", err)
	}
	defer func() {
		if err := sr.Stop(cleanupCtx); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to stop screen recorder: ", err)
		}
		if s.HasError() {
			if err := sr.SaveInBytes(cleanupCtx, filepath.Join(s.OutDir(), "recording.webm")); err != nil {
				testing.ContextLog(cleanupCtx, "Failed to save in bytes: ", err)
			}
		}
		sr.Release(cleanupCtx)
	}()

	// Setup ARC device and UI Automator.
	arcDevice, uiAutomator, err := setUpARC(ctx, cr, s.OutDir())
	if err != nil {
		s.Fatal("Failed to setup ARC: ", err)
	}
	defer arcDevice.Close(cleanupCtx)
	defer uiAutomator.Close(cleanupCtx)

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeSharesheetTest.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files app: ", err)
	}

	// The Sharesheet appears to not properly update the accessibility tree with
	// the coordinates whilst animating. The total time to animate is currently 150ms
	// so setting to 1s to ensure low-end devices are given enough time.
	sharesheet := uiauto.New(tconn).WithInterval(time.Second)
	sharesheetTargetButton := nodewith.Role(role.Button).NameContaining(appShareLabel).ClassName("SharesheetTargetButton")

	if err := uiauto.Combine("Open Downloads and Click sharesheet",
		files.OpenDownloads(),
		files.ClickContextMenuItem(expectedFileName, filesapp.Share),
		sharesheet.LeftClick(sharesheetTargetButton),
	)(ctx); err != nil {
		s.Fatal("Failed to open downloads and click share button: ", err)
	}

	// Wait for the file contents to show in the Android test app.
	fileContentField := uiAutomator.Object(ui.ID(fileContentsID), ui.Text(expectedFileContents))
	if err := fileContentField.WaitForExists(ctx, 15*time.Second); err != nil {
		s.Fatalf("Failed to wait for file contents %q to appear in ARC window: %v", expectedFileContents, err)
	}
}

// setUpARC starts an ARC device and starts UI automator.
func setUpARC(ctx context.Context, cr *chrome.Chrome, outDir string) (*arc.ARC, *ui.Device, error) {
	// Setup ARC device.
	arcDevice, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start ARC")
	}

	// Start up UI automator.
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close UI automator: ", err)
		}
		return nil, nil, errors.Wrap(err, "failed to initialize UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for intent helper")
	}

	return arcDevice, uiAutomator, nil
}
