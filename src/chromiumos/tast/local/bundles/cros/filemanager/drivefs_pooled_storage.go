// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

type testCase struct {
	user                      string
	pass                      string
	expectedBannerClass       string
	expectedStorageMeterClass string
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
			"group:drivefs-cq",
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
				user:                      "filemanager.DrivefsPooledStorage.WarnUsername",
				pass:                      "filemanager.DrivefsPooledStorage.WarnPassword",
				expectedBannerClass:       "tast-drive-low-individual-space",
				expectedStorageMeterClass: "",
			},
		}, {
			Name: "full_individual",
			Val: testCase{
				user:                      "filemanager.DrivefsPooledStorage.FullUsername",
				pass:                      "filemanager.DrivefsPooledStorage.FullPassword",
				expectedBannerClass:       "tast-drive-out-of-individual-space",
				expectedStorageMeterClass: "tast-storage-meter-empty",
			},
		}, {
			Name: "full_organization",
			Val: testCase{
				user:                      "filemanager.DrivefsPooledStorage.OrgFullUsername",
				pass:                      "filemanager.DrivefsPooledStorage.OrgFullPassword",
				expectedBannerClass:       "tast-drive-out-of-organization-space",
				expectedStorageMeterClass: "",
			},
		}},
	})
}

func DrivefsPooledStorage(ctx context.Context, s *testing.State) {
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
	defer cr.Close(cleanupCtx)

	driveFsClient, err := drivefs.NewDriveFs(ctx, s.RequiredVar(tc.user))
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")
	defer driveFsClient.SaveLogsOnError(cleanupCtx, s.HasError)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	if tc.expectedStorageMeterClass != "" {
		// Read storage meter.
		_, err := files.Info(
			ctx,
			nodewith.HasClass(tc.expectedStorageMeterClass).First())
		if err != nil {
			s.Fatalf("Error retrieving storage meter with expected class %q. %v", tc.expectedStorageMeterClass, err)
		}
	}

	if tc.expectedBannerClass != "" {
		// Read active banner.
		_, err := files.Info(
			ctx,
			nodewith.HasClass(tc.expectedBannerClass).Role("banner").First())

		if err != nil {
			s.Fatalf("Error retrieving banner with expected class %q. %v", tc.expectedBannerClass, err)
		}
	}
}
