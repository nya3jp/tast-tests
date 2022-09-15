// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WallpaperImage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of WallpaperImage policy, set the policy to a monochromatic wallpaper then take a screenshot of the desktop wallpaper and check the pixels percentage",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Data:         []string{"wallpaper_image.jpeg"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.WallpaperImage{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// getImgBytesFromFilePath returns bytes of the image with the filePath.
func getImgBytesFromFilePath(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, image, nil)
	if err != nil {
		return nil, err
	}
	imgBytes := buf.Bytes()
	return imgBytes, nil
}

// WallpaperImage tests the WallpaperImage policy.
func WallpaperImage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Check if the current device is in tablet mode.
	tablet, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check tablet mode: ", err)
	}

	red := color.RGBA{255, 0, 0, 255}
	blurredRed := color.RGBA{77, 26, 29, 255}
	expectedPercent := 85
	// The background color in tablets is a bit different.
	// Since the launcher is always open in tablet mode and the apps icons took some space, the expected percentage is reduce to 70%.
	if tablet {
		red = color.RGBA{165, 13, 14, 255}
		blurredRed = color.RGBA{83, 32, 31, 255}
		expectedPercent = 70
	}

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		s.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	imgBytes, err := getImgBytesFromFilePath(s.DataPath("wallpaper_image.jpeg"))
	if err != nil {
		s.Fatal("Failed to read wallpaper image: ", err)
	}
	iurl, ihash := eds.ServePolicyData(imgBytes)

	for _, param := range []struct {
		name                string
		wantChangeWallpaper bool                   // wantChangeWallpaper is the a flag to check if it's allowed to change the wallpaper.
		wantImageCheck      bool                   // wantImageCheck is a flag to check the image pixels.
		value               *policy.WallpaperImage // value is the value of the policy.
	}{
		{
			name:                "nonempty",
			wantChangeWallpaper: false,
			wantImageCheck:      true,
			value:               &policy.WallpaperImage{Val: &policy.WallpaperImageValue{Url: iurl, Hash: ihash}},
		},
		{
			name:                "unset",
			wantChangeWallpaper: true,
			wantImageCheck:      false,
			value:               &policy.WallpaperImage{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if param.wantImageCheck {
				// Check red percentage of the desktop.
				if err := wallpaper.ValidateBackground(cr, red, expectedPercent)(ctx); err != nil {
					s.Error("Failed to validate wallpaper: ", err)
				}
			}

			ui := uiauto.New(tconn)
			if err := personalization.OpenPersonalizationHub(ui)(ctx); err != nil {
				s.Error("Failed to open the personalization hub: ", err)
			}
			if param.wantChangeWallpaper {
				if err := ui.WaitUntilExists(personalization.ChangeWallpaperButton)(ctx); err != nil {
					s.Fatal("Failed to find change wallpaper button when it must exist: ", err)
				}
			} else {
				if err := ui.Gone(personalization.ChangeWallpaperButton)(ctx); err != nil {
					s.Fatal("Failed to ensure that change wallpaper button is gone: ", err)
				}
			}

			if param.wantImageCheck {
				// Lock the screen and check blurred red percentage.
				if err := lockscreen.Lock(ctx, tconn); err != nil {
					s.Fatal("Failed to lock the screen: ", err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
					s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
				}
				if err := wallpaper.ValidateBackground(cr, blurredRed, expectedPercent)(ctx); err != nil {
					s.Error("Failed to validate wallpaper on lock screen: ", err)
				}

				if err := lockscreen.EnterPassword(ctx, tconn, fixtures.Username, fixtures.Password, kb); err != nil {
					s.Fatal("Failed to unlock the screen: ", err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
					s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
				}
			}
		})
	}
}
