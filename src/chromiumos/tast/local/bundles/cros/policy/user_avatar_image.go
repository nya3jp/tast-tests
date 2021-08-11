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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserAvatarImage,
		Desc: "Behavior of UserAvatarImage policy: verify that the user cannot change the device account image when the policy is set, otherwise, the user can change it",
		Contacts: []string{
			"mgawad@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"user_avatar_image.jpeg"},
	})
}

func UserAvatarImage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
	imgBytes, err := getImgFromFilePath(s.DataPath("user_avatar_image.jpeg"))
	if err != nil {
		s.Fatal("Failed to read user avatar image: ", err)
	}
	imgURL, imgHash := eds.ServePolicyData(imgBytes)

	// Get Display Scale Factor to use it to convert bounds in dip to pixels.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	mode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode of the primary display: ", err)
	}
	deviceScaleFactor := mode.DeviceScaleFactor

	timeout := 5 * time.Second

	for _, param := range []struct {
		name                   string
		shouldMatchPolicyImage bool                    // shouldMatchPolicyImage is a flag to check the image pixels.
		value                  *policy.UserAvatarImage // value is the value of the policy.
	}{
		{
			name:                   "non_empty",
			shouldMatchPolicyImage: true,
			value:                  &policy.UserAvatarImage{Val: &policy.UserAvatarImageValue{Url: imgURL, Hash: imgHash}},
		},
		{
			name:                   "unset",
			shouldMatchPolicyImage: false,
			value:                  &policy.UserAvatarImage{Stat: policy.StatusUnset},
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

			ui := uiauto.New(tconn)
			userImagePreviewNode := nodewith.Name("User image preview").Role(role.Image)

			// Open the OS settings changePicture page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/changePicture")
			if err != nil {
				s.Fatal("Failed to open the changePicture page: ", err)
			}
			defer conn.Close()

			// Click on the Cat image, the user image preview should be changed immediately regardless of the policy existence.
			// If the policy is set, the setting is not applied after exiting the view.
			if err := uiauto.Combine("Click on Cat avatar image and wait until the image preview is updated",
				ui.LeftClick(nodewith.Name("Cat").Role(role.RadioButton)),
				ui.WithTimeout(timeout).WaitUntilExists(userImagePreviewNode),
			)(ctx); err != nil {
				s.Fatal("Failed to click on Cat avatar image and wait until the image preview is updated: ", err)
			}

			// Determine the bounds of the user image preview.
			loc, err := ui.Location(ctx, userImagePreviewNode)
			if err != nil {
				s.Fatal("Failed to locate of user image preview")
			}
			rect := coords.ConvertBoundsFromDPToPX(*loc, deviceScaleFactor)

			// Take a screenshot of the user image preview.
			catImageScreenshot, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
			if err != nil {
				s.Fatal("Failed to grap a screenshot of the user image preview")
			}

			// Exit the current view (by clicking Back button) and enter changePicture page again.
			// By exiting and opening the page again, we check if the user image preview was changed or not.
			if err := uiauto.Combine("Click Back and enter changePicture page again",
				ui.LeftClick(nodewith.Name("Change device account image subpage back button").Role(role.Button)),
				ui.WithTimeout(timeout).WaitUntilGone(userImagePreviewNode),
				ui.LeftClick(nodewith.Name("Change device account image").Role(role.Link)),
				ui.WithTimeout(timeout).WaitUntilExists(userImagePreviewNode),
			)(ctx); err != nil {
				s.Fatal("Failed to click Back and enter changePicture page again: ", err)
			}

			// Take a screenshot of the user image preview to check if it was changed or not.
			userImageScreenshot, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
			if err != nil {
				s.Fatal("Failed to grab screenshot: ", err)
			}

			if param.shouldMatchPolicyImage {
				// Verify that the user image is the policy-provided one (red image).
				prcnt := getRedColorPercentage(userImageScreenshot)
				if prcnt < 95 {
					s.Errorf("User image preview doesn't match the policy-provided image: Red pixels percentage: %d", prcnt)
				}
			} else {
				// Verify that the device account image has changed to Cat by the user.
				sim, err := getSimilarityPercentage(catImageScreenshot, userImageScreenshot)
				if err != nil {
					s.Fatal("Failed to count images simialrity percentage: ", err)
				}
				if sim < 95 {
					s.Errorf("User cannot change device account image: Similarity percentage: %d", sim)
				}
			}
		})
	}
}

// getImgFromFilePath returns bytes of the image with the filePath.
// TODO(crbug.com/1188690): Remove when the bug is fixed.
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
	sim := imgcmp.CountPixelsWithDiff(img, red, 60)

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
