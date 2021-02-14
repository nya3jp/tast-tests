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
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WallpaperImage,
		Desc: "Behavior of WallpaperImage policy, set the policy to a monochromatic wallpaper then take a screenshot of the desktop wallpaper and check the pixels percentage",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
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

// countPixelsWithDiff returns how many pixels in the specified color are contained in image with max diff.
func countPixelsWithDiff(image image.Image, clr color.Color, colorMaxDiff uint8) int {
	// TODO(mohamedaomar) crbug/1178509: refactor this function later to a shared package between  policy tests and arc.
	rect := image.Bounds()
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), clr, colorMaxDiff) {
				numPixels++
			}
		}
	}
	return numPixels
}

// grabScreenshot creates a screenshot and returns an image.Image.
func grabScreenshot(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
	// TODO(mohamedaomar) crbug/1178509: refactor this function later to a shared package between  policy tests and arc.
	fd, err := ioutil.TempFile("", "screenshot")
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer os.Remove(fd.Name())
	defer fd.Close()

	if err := screenshot.CaptureChrome(ctx, cr, fd.Name()); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}

// WallpaperImage tests the WallpaperImage policy.
func WallpaperImage(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
			name:            "unset",
			wantRestriction: ui.RestrictionNone,
			wantImageCheck:  false,
			value:           &policy.WallpaperImage{Stat: policy.StatusUnset},
		},
		{
			name:            "nonempty",
			wantRestriction: ui.RestrictionDisabled,
			wantImageCheck:  true,
			value:           &policy.WallpaperImage{Val: &policy.WallpaperImageValue{Url: iurl, Hash: ihash}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if param.wantImageCheck {
				// Take a screenshot and check the red pixels percentage.
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					img, err := grabScreenshot(ctx, cr)
					if err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to grab screenshot"))
					}
					rect := img.Bounds()
					redPixels := countPixelsWithDiff(img, color.RGBA{255, 0, 0, 255}, 10)
					totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
					percent := redPixels * 100 / totalPixels
					if percent < 90 {
						return errors.Errorf("unexpected red pixels percentage: got %d / %d = %d%%; want at least 90%%", redPixels, totalPixels, percent)
					}
					return nil
				}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
					s.Fatal("Did not reach expected state: ", err)
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
		})
	}
}
