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
		Desc: "Behavior of WallpaperImage policy",
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

// getImageFromFilePath returns image with the filePath.
func getImageFromFilePath(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	image, _, err := image.Decode(f)
	return image, err
}

// countRedPercentage returns red pixels percentage in image.
func countRedPercentage(image image.Image) int {
	const colorMaxDiff = 10
	clr := color.RGBA{255, 0, 0, 255}
	rect := image.Bounds()
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), clr, colorMaxDiff) {
				numPixels++
			}
		}
	}
	totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
	percent := numPixels * 100 / totalPixels
	return percent
}

// grabScreenshot creates a screenshot and returns an image.Image.
// The path of the image is generated ramdomly in /tmp.
func grabScreenshot(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
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

	image, err := getImageFromFilePath(s.DataPath("wallpaper_image.jpeg"))
	if err != nil {
		s.Fatal("Failed to read wallpaper image: ", err)
	}
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, image, nil)
	if err != nil {
		s.Fatal("Failed to encode wallpaper image: ", err)
	}
	imgBytes := buf.Bytes()
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
					percent := countRedPercentage(img)
					if percent < 90 {
						return errors.Errorf("unexpected red pixels percentage: got %d%%; want more than 90%%", percent)
					}
					return nil
				}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
					s.Fatal("Did not reach expected state: ", err)
				}
			}

			// Open the personalization page.
			conn, err := cr.NewConn(ctx, "chrome://os-settings/personalization")
			if err != nil {
				s.Fatal("Failed to connect to the personalization setting page: ", err)
			}
			defer conn.Close()

			// Find the Wallpaper app link node.
			wNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Wallpaper Open the wallpaper app",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Could not find Wallpaper app link node: ", err)
			}
			defer wNode.Release(ctx)

			// Check wallpaper app restriction.
			if wNode.Restriction != param.wantRestriction {
				s.Errorf("Unexpected restriction: got %#v; want %#v", wNode.Restriction, param.wantRestriction)
			}
		})
	}
}
