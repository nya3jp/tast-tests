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
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/galleryapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const testFile = "gear_wheels_4000x3000_20200624.jpg"

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchGallery,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Launch Gallery APP on opening supported files",
		Contacts: []string{
			"backlight-swe@google.com",
			"shengjun@chromium.org",
		},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{testFile},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.LoggedIn,
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				ExtraAttr:         []string{"group:mainline"},
			}, {
				Name:    "unstable",
				Fixture: fixture.LoggedIn,
				// b:238260020 - disable aged (>1y) unpromoted informational tests
				// ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			}, {
				Name:              "lacros",
				Fixture:           fixture.LacrosLoggedIn,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"group:mainline"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			},
		},
	})
}

// LaunchGallery verifies launching Gallery on opening supported files.
func LaunchGallery(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithInterval(time.Second)
	//TODO(crbug/1146196) Remove retry after Downloads mounting issue fixed.
	// Setup the test image.

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	// Use the test name to unique name the local test image file.
	// Otherwise the following tests sharing the same Chrome session might have name conflicts.
	// e.g. http://b/198381192.
	localFile := "launch_gallery" + testFile
	localFileLocation := filepath.Join(downloadsPath, localFile)
	if err := ui.Retry(10, func(context.Context) error {
		return fsutil.CopyFile(s.DataPath(testFile), localFileLocation)
	})(ctx); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", localFileLocation, err)
	}
	defer os.Remove(localFileLocation)

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
		files.WithTimeout(30*time.Second).WaitForFile(localFile),
		files.OpenFile(localFile),
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
	imageElementFinder := nodewith.Role(role.Image).Name(localFile).Ancestor(galleryapp.RootFinder)
	if err := ui.WaitUntilExists(imageElementFinder)(ctx); err != nil {
		s.Fatal("Failed to render Gallery: ", err)
	}
}
