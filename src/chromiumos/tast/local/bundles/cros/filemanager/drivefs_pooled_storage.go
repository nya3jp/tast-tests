// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

type testCase struct {
	user                     string
	pass                     string
	expectedBannerText       string
	expectedVisualSignalText string
	expectedStorageMeterText string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsPooledStorage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the Files App UI correctly reflects the DriveFs states related to Pooled Storage",
		Contacts: []string{
			"msalomao@google.org",
			"chromeos-files-syd@google.com",
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
		Params: []testing.Param{{
			Name: "low",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.WarnUsername",
				pass:                     "filemanager.DrivefsPooledStorage.WarnPassword",
				expectedBannerText:       "Storage low 15% left of your 30 GB individual storage.",
				expectedVisualSignalText: "",
				expectedStorageMeterText: "",
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "full_individual",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.FullUsername",
				pass:                     "filemanager.DrivefsPooledStorage.FullPassword",
				expectedBannerText:       "Warning: You’ve used all your individual Google Workspace storage.",
				expectedVisualSignalText: "There is not enough free space in your Google Drive to complete the upload.",
				expectedStorageMeterText: "0 bytes available",
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "full_organization",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.OrgFullUsername",
				pass:                     "filemanager.DrivefsPooledStorage.OrgFullPassword",
				expectedBannerText:       "Warning: Test Org has used all of its Google Workspace storage.",
				expectedVisualSignalText: "Your organization requires more storage to complete the upload.",
				expectedStorageMeterText: "",
			},
			// ExtraData: []string{"sample.h264"},
			ExtraAttr: []string{"informational"},
		}},
		Data: []string{
			"test_1KB.txt",
		},
	})
}

func DrivefsPooledStorage(ctx context.Context, s *testing.State) {
	const testFileName = "test_1KB.txt"

	tc := s.Param().(testCase)

	// Start up Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{
		User: s.RequiredVar(tc.user),
		Pass: s.RequiredVar(tc.pass),
	}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	mountPath, err := drivefs.WaitForDriveFs(ctx, s.RequiredVar(tc.user))
	if err != nil {
		s.Fatal("Failed to wait for DriveFS to be mounted: ", err)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	// Navigate to My Drive and open the "More menu".
	if err := uiauto.Combine("navigate to Drive and open the 'More menu'",
		files.OpenDrive(),
		files.ClickMoreMenuItem(),
	)(ctx); err != nil {
		s.Fatal("Failed to navigate to My drive and open the 'More menu': ", err)
	}

	if tc.expectedStorageMeterText != "" {
		// Read storage meter.
		storageMeter, err := files.Info(
			ctx,
			nodewith.Ancestor(nodewith.HasClass("chrome-menu files-menu")).NameRegex(regexp.MustCompile(" ((used)|(available))$")).First())
		if err != nil {
			s.Fatal("Error retrieving storage meter: ", err)
		} else if storageMeter.Name != tc.expectedStorageMeterText {
			s.Fatalf("Unexpected storage meter contents. Expected %q, found: %q", tc.expectedStorageMeterText, storageMeter.Name)
		}
	}

	if tc.expectedBannerText != "" {
		// Read active banner.
		_, err := files.Info(
			ctx,
			nodewith.Ancestor(nodewith.HasClass("warning-message")).Name(tc.expectedBannerText).First())

		if err != nil {
			s.Fatalf("Error retrieving banner with expected contents %q. %v", tc.expectedBannerText, err)
		}
	}

	if tc.expectedVisualSignalText != "" {
		// Read syncing error visual signal.
		_, err := files.Info(
			ctx,
			nodewith.Ancestor(nodewith.HasClass("files-feedback-panels")).Name(tc.expectedVisualSignalText).First())
		if err != nil {
			s.Fatalf("Error retrieving banner with expected contents %q. %v", tc.expectedVisualSignalText, err)
		}
	}
}
