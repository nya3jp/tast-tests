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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

const testFile = "gear_wheels_4000*3000_20200624.jpg"

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchGallery,
		Desc: "Launch Gallery APP on opening supported files",
		Contacts: []string{
			"backlight-swe@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile},
		Params: []testing.Param{
			{
				Name: "clamshell",
				Val:  false,
			},
			{
				Name: "tablet",
				Val:  true,
			},
		},
	})
}

// LaunchGallery verifies launching Gallery on opening supported files.
func LaunchGallery(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=MediaApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Setup the test image.
	testFileLocation := filepath.Join(filesapp.DownloadPath, testFile)
	if err := fsutil.CopyFile(s.DataPath(testFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	isTabletEnabled := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletEnabled)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Root.Release(ctx)

	// Open the Downloads folder and check for the test file.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}
	if err := files.WaitForFile(ctx, testFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	if err := files.OpenFile(ctx, testFile); err != nil {
		s.Fatal("Waiting to select test file failed: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	// Use image section to verify Gallery App rendering.
	params := ui.FindParams{
		Role: ui.RoleTypeImage,
		Name: testFile,
	}
	if _, err = ui.FindWithTimeout(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to render Gallery or open image: ", err)
	}
}
