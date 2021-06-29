// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/galleryapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const testFile = "gear_wheels_4000x3000_20200624.jpg"

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchGallery,
		Desc: "Launch Gallery APP on opening supported files",
		Contacts: []string{
			"backlight-swe@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{testFile},
		Fixture:      "chromeLoggedInForEA",
		Params: []testing.Param{
			{
				Name: "stable",
				// Test is flaking on betty-release b/190742769.
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			},
		},
	})
}

// LaunchGallery verifies launching Gallery on opening supported files.
func LaunchGallery(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithInterval(time.Second)
	//TODO(crbug/1146196) Remove retry after Downloads mounting issue fixed.
	// Setup the test image.
	testFileLocation := filepath.Join(filesapp.DownloadPath, testFile)
	if err := ui.Retry(10, func(context.Context) error {
		return fsutil.CopyFile(s.DataPath(testFile), testFileLocation)
	})(ctx); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// SWA installation is not guaranteed during startup.
	// Using this wait to check installation finished before starting test.
	s.Log("Wait for Gallery to be installed")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Gallery.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	if err := uiauto.Combine("open Downloads folder and double click file to launch Gallery",
		files.OpenDownloads(),
		files.WithTimeout(30*time.Second).WaitForFile(testFile),
		files.OpenFile(testFile),
	)(ctx); err != nil {
		s.Fatal("Failed to open file in Downloads: ", err)
	}

	s.Log("Wait for Gallery shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	s.Log("Wait for Gallery app rendering")
	// Use image section to verify Gallery App rendering.
	ui = uiauto.New(tconn).WithTimeout(time.Minute)
	imageElementFinder := nodewith.Role(role.Image).Name(testFile).Ancestor(galleryapp.RootFinder)
	if err := ui.WaitUntilExists(imageElementFinder)(ctx); err != nil {
		s.Fatal("Failed to render Gallery: ", err)
	}

	s.Log("Delete opened media file and assert zero state")
	gc := galleryapp.NewContext(cr, tconn)
	if err := uiauto.Combine("delete file in app and verify it is removed from local drive",
		gc.DeleteAndConfirm(),
		gc.AssertZeroState(),
		// CloseApp is a necessary step before checking file gone.
		// files.WaitUntilFileGone checks files A11y tree.
		// However fileApp A11y tree will not update until it is brought to front.
		gc.CloseApp(),
		files.WaitUntilFileGone(testFile),
	)(ctx); err != nil {
		s.Fatal("Failed to remove media file in app: ", err)
	}
}
