// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WallpaperImage,
		Desc: "Behavior of WallpaperImage policy, set the policy to a monochromatic wallpaper then take a screenshot of the desktop wallpaper and check the pixels percentage",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"wallpaper_image.jpeg"},
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

// validateBackground takes a screenshot and check the percentage of the clr in the image, returns error if it's less than expectedPercent%.
func validateBackground(ctx context.Context, cr *chrome.Chrome, clr color.Color, expectedPercent int) error {
	// Take a screenshot and check the red pixels percentage.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to grab screenshot")
		}
		rect := img.Bounds()
		redPixels := imgcmp.CountPixelsWithDiff(img, clr, 60)
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := redPixels * 100 / totalPixels
		if percent < expectedPercent {
			return errors.Errorf("unexpected red pixels percentage: got %d / %d = %d%%; want at least %d%%", redPixels, totalPixels, percent, expectedPercent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}

// WallpaperImage tests the WallpaperImage policy.
func WallpaperImage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
		name            string
		wantRestriction ui.RestrictionState    // wantRestriction is the wanted restriction state of the wallpaper app link.
		wantImageCheck  bool                   // wantImageCheck is a flag to check the image pixels.
		value           *policy.WallpaperImage // value is the value of the policy.
	}{
		{
			name:            "nonempty",
			wantRestriction: ui.RestrictionDisabled,
			wantImageCheck:  true,
			value:           &policy.WallpaperImage{Val: &policy.WallpaperImageValue{Url: iurl, Hash: ihash}},
		},
		{
			name:            "unset",
			wantRestriction: ui.RestrictionNone,
			wantImageCheck:  false,
			value:           &policy.WallpaperImage{Stat: policy.StatusUnset},
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
				if err := validateBackground(ctx, cr, red, expectedPercent); err != nil {
					s.Error("Failed to validate wallpaper: ", err)
				}
			}

			// Open the os settings personalization page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/personalization")
			if err != nil {
				s.Fatal("Failed to open the personalization setting page: ", err)
			}
			defer conn.Close()

			// Find the Wallpaper app link node.
			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeLink,
					Name: "Wallpaper Open the wallpaper app",
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			if param.wantImageCheck {
				// Lock the screen and check blurred red percentage.
				if err := lockscreen.Lock(ctx, tconn); err != nil {
					s.Fatal("Failed to lock the screen: ", err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
					s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
				}
				if err := validateBackground(ctx, cr, blurredRed, expectedPercent); err != nil {
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
