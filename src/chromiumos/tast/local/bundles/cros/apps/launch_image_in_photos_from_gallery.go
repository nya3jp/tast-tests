// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/arc"
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

// todo: move to shared package to share with launch_gallery.go? galleryapp.go?
const testFile2 = "gear_wheels_4000x3000_20200624.jpg"

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchImageInPhotosFromGallery,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "From the Gallery app, launch an opened image in the Photos app",
		Contacts: []string{
			"backlight-swe@google.com",
			"bugsnash@chromium.org",
			"shengjun@google.com",
		},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{testFile2},
		Params: []testing.Param{
			// todo: fix issues with unstable and lacros. probably need to create a new fixture for lacrosWithArcBootedAndMediaAppPhotosIntegrationImageFeatureFlag
			{
				Name:              "stable",
				Fixture:           "arcBootedWithMediaAppPhotosIntegrationImageFeatureFlag",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				Fixture:           "arcBootedWithMediaAppPhotosIntegrationImageFeatureFlag",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			}, {
				Name:              "lacros",
				Fixture:           "lacrosWithArcBooted",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			},
		},
	})
}

// LaunchImageInPhotosFromGallery verifies launching Photos from Gallery on an open image file.
func LaunchImageInPhotosFromGallery(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
	localFile := "launch_image_in_photos_from_gallery" + testFile2
	localFileLocation := filepath.Join(downloadsPath, localFile)
	if err := ui.Retry(10, func(context.Context) error {
		return fsutil.CopyFile(s.DataPath(testFile2), localFileLocation)
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

	s.Log("Wait for Photos to be installed")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Photos.ID, 2*time.Minute); err != nil {
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

	s.Log("Click annotation button")
	annotationButtonFinder := nodewith.Role(role.ToggleButton).Name(`Annotate`).Ancestor(galleryapp.RootFinder)
	if err := ui.WaitUntilExists(annotationButtonFinder)(ctx); err != nil {
		s.Fatal("Couldn't find `Annotate` button: ", err)
	}
	if err := ui.LeftClick(annotationButtonFinder)(ctx); err != nil {
		s.Fatal("Failed to click `Annotate` button: ", err)
	}

	s.Log("-----a11y tree:-----")
	s.Log(uiauto.RootDebugInfo(ctx, tconn))

	s.Log("Check visibility of `More tools in Photos` button")
	photosButtonFinder := nodewith.Role(role.Button).Name(`More tools in Photos`).Ancestor(galleryapp.RootFinder)
	if err := ui.WaitUntilExists(photosButtonFinder)(ctx); err != nil {
		s.Fatal("Couldn't find `More tools in Photos` button: ", err)
	}

	// todo: click `More tools in Photos` button

	// todo: wait for photos launch (similar to wait for Gallery) ash function
}
