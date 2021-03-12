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

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserAvatarImage,
		Desc: "Behavior of UserAvatarImage policy",
		Contacts: []string{
			"mgawad@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"user_avatar_image.jpg"},
	})
}

// getImgFromFilePath returns bytes of the image with the filePath.
func getImgFromFilePath(filePath string) ([]byte, error) {
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

func getRedColorPercentage(img image.Image) int {
	red := color.RGBA{255, 0, 0, 255}
	sim := imgcmp.CountPixelsWithDiff(img, red, 10)

	bounds := img.Bounds()
	total := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X)
	prcnt := sim * 100 / total
	return prcnt
}

func getSimilarityPercentage(img1, img2 image.Image) (int, error) {
	diff, err := imgcmp.CountDiffPixels(img1, img2, 10)
	if err != nil {
		return -1, err
	}

	bounds := img1.Bounds()
	total := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X)
	prcnt := 100 - diff*100/total
	return prcnt, nil
}

func UserAvatarImage(ctx context.Context, s *testing.State) {
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

	// Serve UserAvatarImage policy data.
	imgBytes, err := getImgFromFilePath(s.DataPath("user_avatar_image.jpg"))
	if err != nil {
		s.Fatal("Failed to read user avatar image: ", err)
	}
	iurl, ihash := eds.ServePolicyData(imgBytes)

	// Get Display Scale Factor to use it to convert bounds in dip to pixels.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	mode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode of the primary display: ", err)
	}
	dsf := mode.DeviceScaleFactor

	for _, param := range []struct {
		name                   string
		wantRestriction        ui.RestrictionState     // wantRestriction is the wanted restriction state of the wallpaper app link.
		shouldMatchPolicyImage bool                    // shouldMatchPolicyImage is a flag to check the image pixels.
		value                  *policy.UserAvatarImage // value is the value of the policy.
	}{
		{
			name:                   "unset",
			wantRestriction:        ui.RestrictionNone,
			shouldMatchPolicyImage: false,
			value:                  &policy.UserAvatarImage{Stat: policy.StatusUnset},
		},
		{
			name:                   "nonempty",
			wantRestriction:        ui.RestrictionDisabled,
			shouldMatchPolicyImage: true,
			value:                  &policy.UserAvatarImage{Val: &policy.UserAvatarImageValue{Url: iurl, Hash: ihash}},
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

			uiCtx := uiauto.New(tconn)
			userImagePreviewNode := nodewith.Name("User image preview").Role(role.Image)
			changeDeviceAccountImageNode := nodewith.Name("Change device account image").Role(role.Link)

			// Open the os settings personalization page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/personalization")
			if err != nil {
				s.Fatal("Failed to open the personalization setting page: ", err)
			}
			defer conn.Close()

			if err := uiauto.Combine("choose Cat photo",
				uiCtx.LeftClick(changeDeviceAccountImageNode),
				uiCtx.LeftClick(nodewith.Name("Cat").Role(role.RadioButton)),
				uiCtx.WaitUntilExists(userImagePreviewNode),
			)(ctx); err != nil {
				s.Fatal("Failed to choose Cat photo: ", err)
			}

			// Determine the bounds of User image preview
			loc, err := uiCtx.Location(ctx, userImagePreviewNode)
			if err != nil {
				s.Fatal("Failed to locate of User image preview")
			}
			rect := coords.ConvertBoundsFromDPToPX(*loc, dsf)

			// Take a screen shot of the Cat user image
			catImageScreenshot, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
			if err != nil {
				s.Fatal("Failed to grap a screenshot")
			}

			// Open the os settings personalization page again.
			conn, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/personalization")
			if err != nil {
				s.Fatal("Failed to open the personalization setting page: ", err)
			}

			if err := uiauto.Combine("navigate to User image preview",
				uiCtx.LeftClick(changeDeviceAccountImageNode),
				uiCtx.WaitUntilExists(userImagePreviewNode),
			)(ctx); err != nil {
				s.Fatal("Failed to navigate to User image preview: ", err)
			}

			// Take a screen shot of the current user image
			userImageScreenshot, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
			if err != nil {
				s.Fatal("Failed to grab screenshot: ", err)
			}

			if param.shouldMatchPolicyImage {
				// Verify that the user image is the policy-provided one (red image).
				prcnt := getRedColorPercentage(userImageScreenshot)
				if prcnt < 90 {
					s.Fatal("User image preview doesn't match the policy-provided image: ", errors.Wrapf(nil, "Red Pixels Percentage: %d", prcnt))
				}
			} else {
				// Verify that the user can change device account image.
				sim, err := getSimilarityPercentage(catImageScreenshot, userImageScreenshot)
				if err != nil {
					s.Fatal("Failed to count images simialrity percentage: ", err)
				}
				if sim < 95 {
					s.Fatal("User cannot change device account image: ", errors.Wrapf(nil, "Similarity percentage: %d", sim))
				}
			}
		})
	}
}
