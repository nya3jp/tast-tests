// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsPooledStorage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the Files App UI correctly reflects the DriveFs states related to Pooled Storage",
		Contacts: []string{
			"msalomao@google.org",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Attr: []string{
			// TODO: promote to "group:drivefs-cq",
			"informational",
			"group:mainline",
		},
		VarDeps: []string{
			"filemanager.DrivefsPooledStorage.OrgFullUsername",
			"filemanager.DrivefsPooledStorage.OrgFullPassword",
			"filemanager.DrivefsPooledStorage.FullUsername",
			"filemanager.DrivefsPooledStorage.FullPassword",
			"filemanager.DrivefsPooledStorage.WarnUsername",
			"filemanager.DrivefsPooledStorage.WarnPassword",
		},
		Data: []string{
			"test_1KB.txt",
		},
	})
}

type testCase struct {
	user               string
	pass               string
	isQuotaRunningLow  bool
	isIndividualFull   bool
	isOrganizationFull bool
}

func DrivefsPooledStorage(ctx context.Context, s *testing.State) {
	const testFileName = "test_1KB.txt"

	for _, tc := range []testCase{
		{
			user:               s.RequiredVar("filemanager.DrivefsPooledStorage.WarnUsername"),
			pass:               s.RequiredVar("filemanager.DrivefsPooledStorage.WarnPassword"),
			isQuotaRunningLow:  true,
			isIndividualFull:   false,
			isOrganizationFull: false,
		},
		{
			user:               s.RequiredVar("filemanager.DrivefsPooledStorage.FullUsername"),
			pass:               s.RequiredVar("filemanager.DrivefsPooledStorage.FullPassword"),
			isQuotaRunningLow:  false,
			isIndividualFull:   true,
			isOrganizationFull: false,
		},
		{
			user:               s.RequiredVar("filemanager.DrivefsPooledStorage.OrgFullUsername"),
			pass:               s.RequiredVar("filemanager.DrivefsPooledStorage.OrgFullPassword"),
			isQuotaRunningLow:  false,
			isIndividualFull:   false,
			isOrganizationFull: true,
		},
	} {
		// Start up Chrome.
		cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{
			User: tc.user,
			Pass: tc.pass,
		}))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		mountPath, err := drivefs.WaitForDriveFs(ctx, tc.user)
		if err != nil {
			s.Fatal("Failed to wait for DriveFS to be mounted: ", err)
		}

		// Open the test API.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API Connection: ", err)
		}
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		drivefsRoot := filepath.Join(mountPath, "root")

		// Copy the test file into My Drive.
		if err := fsutil.CopyFile(s.DataPath(testFileName), filepath.Join(drivefsRoot, testFileName)); err != nil {
			s.Fatalf("Cannot copy %q to %q: %v", testFileName, drivefsRoot, err)
		}

		// Launch Files App.
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Files app: ", err)
		}

		ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

		// Navigate to My Drive and open the "More menu".
		files.ClickMoreMenuItem()
		if err := uiauto.Combine("open downloads and check items",
			files.OpenDrive(),
			files.ClickMoreMenuItem(),
		)(ctx); err != nil {
			s.Fatal("Failed to navigate to My drive and open the 'More menu': ", err)
		}

		// Read storage meter.
		storageMeter, err := ui.Info(
			ctx,
			nodewith.Ancestor(nodewith.HasClass("chrome-menu files-menu")).NameRegex(regexp.MustCompile(" ((used)|(available))$")).Role(role.MenuItem).First())
		if tc.isIndividualFull && storageMeter.Name != "0 bytes available" {
			s.Fatal("User should have no storage left")
		}

		if tc.isOrganizationFull {
			// Read active banner.
			_, err := ui.Info(
				ctx,
				nodewith.Ancestor(nodewith.HasClass("warning-message")).Name("Warning: Test Org has used all of its Google Workspace storage.").Role(role.GenericContainer))

			if err != nil {
				s.Fatal("User should have received banner informing their individual quota is full: ", err)
			}

			// Read syncing error visual signal.
			_, err = ui.Info(
				ctx,
				nodewith.Ancestor(nodewith.HasClass("files-feedback-panels")).Name("Your organization requires more storage to complete the upload.").Role(role.StaticText))
			if err != nil {
				s.Fatal("Drive syncing should have failed with organization quota exceeded error: ", err)
			}
		} else if tc.isIndividualFull {
			// Read active banner.
			_, err := ui.Info(
				ctx,
				nodewith.Ancestor(nodewith.HasClass("warning-message")).Name("Warning: You’ve used all your individual Google Workspace storage.").Role(role.GenericContainer))

			if err != nil {
				s.Fatal("User should have received banner informing their organization quota is full: ", err)
			}

			// Read syncing error visual signal.
			_, err = ui.Info(
				ctx,
				nodewith.Ancestor(nodewith.HasClass("files-feedback-panels")).Name("There is not enough free space in your Google Drive to complete the upload.").Role(role.StaticText))
			if err != nil {
				s.Fatal("Drive syncing should have failed with individual quota exceeded error: ", err)
			}
		} else if tc.isQuotaRunningLow {
			// Read active banner.
			_, err := ui.Info(
				ctx,
				nodewith.Ancestor(nodewith.HasClass("warning-message")).NameContaining("% left of your 30 GB individual storage.").Role(role.InlineTextBox))

			if err != nil {
				s.Fatal("User should have received warning about their quota running low: ", err)
			}
		}
	}
}
