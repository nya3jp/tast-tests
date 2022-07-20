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
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/apps/galleryapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const testImageFileWithText = "happy_halloween_20220721.png"

var textBlockInImage = []string{"HAPPY", "HALLOWEEN"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchImageInPhotosFromGallery,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "From the Gallery app, launch an opened image in the Photos app",
		Contacts: []string{
			"backlight-swe@google.com",
			"bugsnash@chromium.org",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{testImageFileWithText},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           "arcBootedWithGalleryPhotosImageFeature",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				Fixture:           "arcBootedWithGalleryPhotosImageFeature",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			},
		},
	})
}

// LaunchImageInPhotosFromGallery verifies launching Photos from Gallery on an open image file.
func LaunchImageInPhotosFromGallery(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Install Photos app")
	const photosAppPkgName = "com.google.android.apps.photos"
	if err := playstore.InstallOrUpdateAppAndClose(ctx, tconn, a, d, photosAppPkgName, &playstore.Options{TryLimit: -1}); err != nil {
		s.Fatal("Failed to install Photos app: ", err)
	}

	// Setup the test image.
	ui := uiauto.New(tconn).WithInterval(time.Second)
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	// Use the test name to unique name the local test image file.
	// Otherwise the following tests sharing the same Chrome session might have name conflicts.
	// e.g. http://b/198381192.
	localFile := "launch_image_in_photos_from_gallery" + testImageFileWithText
	localFileLocation := filepath.Join(downloadsPath, localFile)
	// TODO(crbug/1146196) Remove retry after Downloads mounting issue fixed.
	if err := ui.Retry(10, func(context.Context) error {
		return fsutil.CopyFile(s.DataPath(testImageFileWithText), localFileLocation)
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
	ui = uiauto.New(tconn)

	s.Log("Click `Lighting filters` button")
	annotationButton := nodewith.Role(role.ToggleButton).Name(`Lighting filters`).Ancestor(galleryapp.RootFinder)
	if err := ui.LeftClick(annotationButton)(ctx); err != nil {
		s.Fatal("Failed to click `Lighting filters` button: ", err)
	}

	s.Log("Click `More tools in Photos` button")
	photosButton := nodewith.Role(role.Button).Name(`More tools in Photos`).Ancestor(galleryapp.RootFinder)
	if err := ui.LeftClick(photosButton)(ctx); err != nil {
		s.Fatal("Failed to click `More tools in Photos` button: ", err)
	}

	s.Log("Wait for Photos shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Photos.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Photos in shelf: ", err)
	}

	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute)

	if err := uiauto.NamedCombine("reach main page of Photos app",
		ud.LeftClick(uidetection.Word("ALLOW")),
		ud.LeftClick(uidetection.TextBlock([]string{"Got", "it"})),
		ud.WaitUntilExists(uidetection.TextBlock(textBlockInImage)),
	)(ctx); err != nil {
		s.Fatal("Failed to verify the test image opened in Photos: ", err)
	}
}
