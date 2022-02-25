// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsDssOffline,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify that making a Docs/Sheets/Slides file available offline through Files App works",
		Contacts: []string{
			"austinct@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Timeout: 5 * time.Minute,
		Fixture: "driveFsStartedWithNativeMessaging",
		SearchFlags: []*testing.StringPair{
			{
				Key:   "feature_id",
				Value: "screenplay-32d74807-7b2f-46ae-8bef-b34b76ab328c",
			},
		},
	})
}

func installRequiredExtensions(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	// TODO(b/193595364): Figure out why these extensions aren't being installed by default in tast tests.
	docsOfflineName := "Google Docs Offline"
	docsOfflineURL := "https://chrome.google.com/webstore/detail/google-docs-offline/ghbmnnjooekpmoecnnnilnnbdlolhkhi"
	docsOfflineExt := cws.App{Name: docsOfflineName, URL: docsOfflineURL}
	if err := cws.InstallApp(ctx, cr.Browser(), tconn, docsOfflineExt); err != nil {
		return errors.Wrap(err, "failed to install Google Docs Offline extension")
	}

	proxyExtName := "Application Launcher For Drive (by Google)"
	proxyExtURL := "https://chrome.google.com/webstore/detail/application-launcher-for/lmjegmlicamnimmfhcmpkclmigmmcbeh"
	proxyExt := cws.App{Name: proxyExtName, URL: proxyExtURL}
	if err := cws.InstallApp(ctx, cr.Browser(), tconn, proxyExt); err != nil {
		return errors.Wrap(err, "failed to install Application Launcher for Drive extension")
	}
	return nil
}

func DrivefsDssOffline(ctx context.Context, s *testing.State) {
	APIClient := s.FixtValue().(*drivefs.FixtureData).APIClient
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn
	driveFsClient := s.FixtValue().(*drivefs.FixtureData).DriveFs

	uniqueSuffix := fmt.Sprintf("-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
	testDocFileName := fmt.Sprintf("doc-drivefs%s", uniqueSuffix)
	uniqueTestFolderName := fmt.Sprintf("DrivefsDssOffline%s", uniqueSuffix)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Create the unique folder that will be directly navigated to below.
	testFilePath := driveFsClient.MyDrivePath(uniqueTestFolderName, testDocFileName)
	folder, err := APIClient.Createfolder(ctx, uniqueTestFolderName, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create folder in MyDrive: ", err)
	}
	defer APIClient.RemoveFileByID(cleanupCtx, folder.Id)

	// Create a blank Google doc in the nested folder above.
	file, err := APIClient.CreateBlankGoogleDoc(ctx, testDocFileName, []string{folder.Id})
	if err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}
	defer APIClient.RemoveFileByID(cleanupCtx, file.Id)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer driveFsClient.SaveLogsOnError(cleanupCtx, s.HasError)

	if err := installRequiredExtensions(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to install the required extensions: ", err)
	}

	var filesApp *filesapp.FilesApp
	testFileNameWithExt := fmt.Sprintf("%s.gdoc", testDocFileName)
	// There is a small period of time on startup where DriveFS can't pin Docs files, so repeatedly
	// relaunch Files App until the Available offline toggle is enabled.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Launch Files App directly to the directory containing the `testFilePath`.
		filesApp, err = filesapp.LaunchSWAToPath(ctx, tconn, filepath.Dir(testFilePath))
		if err != nil {
			return err
		}
		filesApp.WaitForFile(testFileNameWithExt)(ctx)
		filesApp.SelectFile(testFileNameWithExt)(ctx)
		nodeInfo, err := filesApp.Info(ctx, nodewith.Name("Available offline").Role(role.ToggleButton))
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the current status of the Available offline toggle"))
		}
		if _, disabled := nodeInfo.HTMLAttributes["disabled"]; disabled {
			filesApp.Close(ctx)
			return errors.New("the Available offline toggle is still disabled")
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond}); err != nil {
		s.Fatal("Failed to wait for the Available offline toggle to be enabled: ", err)
	}

	// Try make the newly created Google doc available offline.
	if err := filesApp.ToggleAvailableOfflineForFile(testFileNameWithExt)(ctx); err != nil {
		s.Fatalf("Failed to make test file %q in Drive available offline: %v", testFileNameWithExt, err)
	}

	// Sometimes offline is already enabled and the enable offline notification does not show,
	// so add a timeout when trying to dismiss the notification.
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(2 * time.Second).LeftClick(nodewith.Name("Enable Offline").Role(role.Button))(ctx); err != nil {
		s.Log("Failed to enable offline, offline might already be enabled")
	}

	// Reselect the file in order to query the Available offline toggle.
	if err := filesApp.SelectFile(testFileNameWithExt)(ctx); err != nil {
		s.Fatalf("Failed to reselect test file %q: %v", testFileNameWithExt, err)
	}

	s.Log("Waiting for the Available offline toggle to stabilize")
	var previousNodeInfo *uiauto.NodeInfo
	var currentNodeInfo *uiauto.NodeInfo
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentNodeInfo, err = filesApp.Info(ctx, nodewith.Name("Available offline").Role(role.ToggleButton))
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get info of the Available offline toggle"))
		}
		if previousNodeInfo != nil && previousNodeInfo.Checked == currentNodeInfo.Checked {
			return nil
		}
		previousNodeInfo = currentNodeInfo
		return errors.New("the Available offline toggle did not stabilize")
	}, &testing.PollOptions{Interval: 500 * time.Millisecond}); err != nil {
		s.Fatal("Failed to wait for the Available offline toggle to stabilize: ", err)
	}

	if currentNodeInfo.Checked != checked.True {
		s.Fatalf("The test file %q in Drive was not made available offline", testFileNameWithExt)
	}
}
