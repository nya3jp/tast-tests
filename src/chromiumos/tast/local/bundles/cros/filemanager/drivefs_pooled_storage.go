// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"regexp"
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
	user                     string
	pass                     string
	expectedBannerText       string
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
			// TODO: promote to "group:drivefs-cq"
			// TODO(https://crbug.com/1355719): reduce the boards this runs on workspace
			// on it, we should probably consider reducing the boards instead.
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
		// TODO(https://crbug.com/1355708): avoid hardcoding i18n strings.
		Params: []testing.Param{{
			Name: "low",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.WarnUsername",
				pass:                     "filemanager.DrivefsPooledStorage.WarnPassword",
				expectedBannerText:       "Storage low 15% left of your 30 GB individual storage.",
				expectedStorageMeterText: "",
			},
		}, {
			Name: "full_individual",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.FullUsername",
				pass:                     "filemanager.DrivefsPooledStorage.FullPassword",
				expectedBannerText:       "Warning: You’ve used all your individual Google Workspace storage.",
				expectedStorageMeterText: "0 bytes available",
			},
		}, {
			Name: "full_organization",
			Val: testCase{
				user:                     "filemanager.DrivefsPooledStorage.OrgFullUsername",
				pass:                     "filemanager.DrivefsPooledStorage.OrgFullPassword",
				expectedBannerText:       "Warning: Test Org has used all of its Google Workspace storage.",
				expectedStorageMeterText: "",
			},
		}},
		Data: []string{
			"test_1KB.txt",
		},
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

	if _, err := drivefs.NewDriveFs(ctx, s.RequiredVar(tc.user)); err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")

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
}
