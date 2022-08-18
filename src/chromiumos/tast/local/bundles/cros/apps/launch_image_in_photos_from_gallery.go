// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
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

const testImageFileWithText = "happy_halloween.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchImageInPhotosFromGallery,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "From the Gallery app, launch an opened image in the Android Photos app",
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
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				Fixture:           "arcBootedWithGalleryPhotosImageFeature",
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
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// This test inherits a parent fixture from ARC++ where screen recorder is not available.
	recorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create screen recorder: ", err)
	} else {
		if recorder.Start(ctx, tconn); err != nil {
			s.Log("Failed to start screen recorder: ", err)
		} else {
			defer recorder.StopAndSaveOnError(cleanupCtx, filepath.Join(s.OutDir(), "record.webm"), s.HasError)
		}
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	// Wait for Gallery to be installed
	// SWA installation is not guaranteed during startup.
	// Using this wait to check installation finished before starting test.
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Gallery.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Close(cleanupCtx)

	if err := uiauto.Combine("open Downloads folder and double click file to launch Gallery",
		files.OpenDownloads(),
		files.WithTimeout(30*time.Second).WaitForFile(localFile),
		files.OpenFile(localFile),
	)(ctx); err != nil {
		s.Fatal("Failed to open file in Downloads: ", err)
	}

	// Wait for Gallery shown in shelf
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	if err := uiauto.Combine("Click on `More tools in Photos` button in Gallery",
		// Use image section to verify Gallery App rendering.
		ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Role(role.Image).Name(localFile).Ancestor(galleryapp.RootFinder)),
		// Click `Lighting filters` to reveal `More tools in Photos` button.
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name(`Lighting filters`).Ancestor(galleryapp.RootFinder)),
		// Click `More tools in Photos` button.
		ui.LeftClick(nodewith.Role(role.Button).Name(`More tools in Photos`).Ancestor(galleryapp.RootFinder)),
	)(ctx); err != nil {
		s.Fatal("Failed to click `More tools in Photos` button in Gallery: ", err)
	}

	// Wait for Photos shown in shelf
	if err := ash.WaitForApp(ctx, tconn, apps.Photos.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Photos in shelf: ", err)
	}

	// Wait for image to appear in Photos app
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute)
	allowButton := uidetection.Word("ALLOW")
	gotItButton := uidetection.TextBlock([]string{"Got", "it"})

	// Only clear the prompt if it shows up within certain time.
	// ARC++ might not be ready to receive CLICK during launch.
	// Using retry to mitigate UI flakiness.
	closeIfShown := func(finder *uidetection.Finder) uiauto.Action {
		return uiauto.IfSuccessThen(
			// Long timeout is required here as the Photos first launch is very slow.
			ud.WithTimeout(10*time.Second).WaitUntilExists(finder),
			uiauto.Retry(3, uiauto.Combine("click button and waits its gone",
				ud.WithTimeout(5*time.Second).LeftClick(finder),
				ud.WithTimeout(5*time.Second).WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilGone(finder),
			)),
		)
	}

	if err := uiauto.NamedCombine("reach main page of Photos app",
		closeIfShown(allowButton),
		closeIfShown(gotItButton),
		ud.WaitUntilExists(uidetection.Word("HALLOWEEN")),
	)(ctx); err != nil {
		s.Fatal("Failed to verify the test image opened in Photos: ", err)
	}
}
