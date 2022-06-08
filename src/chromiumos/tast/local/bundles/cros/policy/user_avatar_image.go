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
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserAvatarImage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of UserAvatarImage policy: verify that the user cannot change the device account image when the policy is set, otherwise, the user can change it",
		Contacts: []string{
			"mgawad@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedInWithoutPersonalizationHub,
		Data:         []string{"user_avatar_image.jpeg"},
	})
}

func UserAvatarImage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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

			// Open the OS settings changePicture page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/changePicture")
			if err != nil {
				s.Fatal("Failed to open the changePicture page: ", err)
			}
			defer conn.Close()

			// Get the list of device account images to select the last one of them.
			ui := uiauto.New(tconn)
			deviceAccountImages, err := ui.NodesInfo(ctx, nodewith.Ancestor(nodewith.Role(role.RadioGroup)))
			if err != nil {
				s.Fatal("Failed to get deviceAccountImages for selector node: ", err)
			}
			avatarImageNodeInfo := deviceAccountImages[len(deviceAccountImages)-1]
			avatarImageNode := nodewith.Role(role.RadioButton).Name(avatarImageNodeInfo.Name)

			// Click on a new avatar image, the user image preview should be changed
			// immediately regardless of the policy existence.
			// If policy is set, the setting is not applied after exiting the view.
			if err := uiauto.Combine("Click on the selected avatar image",
				ui.WithTimeout(timeout).MakeVisible(avatarImageNode),
				ui.LeftClick(avatarImageNode),
			)(ctx); err != nil {
				s.Fatal("Failed to click on the selected avatar image: ", err)
			}

			userImagePreviewNode := nodewith.Name("User image preview").Role(role.Image)

			// Take a screenshot of the user image preview.
			avatarImageScreenshot, err := grapImgNodeScreenshot(ctx, cr, userImagePreviewNode, deviceScaleFactor)
			if err != nil {
				s.Fatal("Failed to grap a screenshot of the user image preview: ", err)
			}

			// Exit the current view (by clicking Back button) and enter changePicture
			// page again.
			// By exiting and opening the page again, we check if the user image
			// preview was changed or not.
			if err := uiauto.Combine("Click Back and enter changePicture page again",
				ui.LeftClick(nodewith.Name("Change device account image subpage back button").Role(role.Button)),
				ui.WithTimeout(timeout).WaitUntilGone(userImagePreviewNode),
				ui.LeftClick(nodewith.Name("Change device account image").Role(role.Link)),
				ui.WithTimeout(timeout).WaitUntilExists(userImagePreviewNode),
			)(ctx); err != nil {
				s.Fatal("Failed to click Back and enter changePicture page again: ", err)
			}

			// Take a screenshot of the user image preview to check if it was changed.
			userImageScreenshot, err := grapImgNodeScreenshot(ctx, cr, userImagePreviewNode, deviceScaleFactor)
			if err != nil {
				s.Fatal("Failed to grab screenshot: ", err)
			}

			if param.shouldMatchPolicyImage {
				// Verify that the user image is the policy-provided one (red image).
				// The image is now cropped to be a circle (filled with ~78% red).
				prcnt := getRedColorPercentage(userImageScreenshot)
				if !(75 < prcnt && prcnt < 81) {
					s.Errorf("User image preview doesn't match the policy-provided image: Red pixels percentage: %d", prcnt)
					if err := saveImage(filepath.Join(s.OutDir(), "red_avatar.jpeg"), userImageScreenshot); err != nil {
						s.Error("Failed to save the avatar image: ", err)
					}
				}
			} else {
				// Verify that the device account image has changed to the selected
				// avatar image by the user.
				userImagePreviewNodeInfo, err := ui.NodesInfo(ctx, userImagePreviewNode)
				if err != nil {
					s.Fatal("Failed to get node info for the user image preview: ", err)
				}

				imagePreviewSrcAttr := userImagePreviewNodeInfo[0].HTMLAttributes["src"]
				avatarImageDataURLAttr := avatarImageNodeInfo.HTMLAttributes["data-url"]

				// Check if both (the image preview and the selected avatar image) come
				// from the same source.
				// Image preview src attribute value contains extra scaling information,
				// for example if avatar image data-url attribute equals to
				// "chrome://theme/IDR_LOGIN_DEFAULT_USER_82", the image preview might
				// equal to "chrome://theme/IDR_LOGIN_DEFAULT_USER_82@2x"
				if !strings.HasPrefix(imagePreviewSrcAttr, avatarImageDataURLAttr) {
					s.Errorf("User cannot change account image, imagePreviewSrcAttr=%s, avatarImageDataURLAttr=%s", imagePreviewSrcAttr, avatarImageDataURLAttr)
					if err := saveImage(filepath.Join(s.OutDir(), "original_avatar.jpeg"), avatarImageScreenshot); err != nil {
						s.Error("Failed to save the original avatar image: ", err)
					}
					if err := saveImage(filepath.Join(s.OutDir(), "updated_avatar.jpeg"), userImageScreenshot); err != nil {
						s.Error("Failed to save the updated avatar image: ", err)
					}
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

func grapImgNodeScreenshot(ctx context.Context, cr *chrome.Chrome, node *nodewith.Finder, deviceScaleFactor float64) (image.Image, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}
	ui := uiauto.New(tconn)

	// Determine the bounds of node.
	loc, err := ui.Location(ctx, node)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the bounds of the image node")
	}
	rect := coords.ConvertBoundsFromDPToPX(*loc, deviceScaleFactor)

	return screenshot.GrabAndCropScreenshot(ctx, cr, rect)
}

func getRedColorPercentage(img image.Image) int {
	red := color.RGBA{255, 0, 0, 255}
	sim := imgcmp.CountPixelsWithDiff(img, red, 60)

	bounds := img.Bounds()
	total := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X)
	prcnt := sim * 100 / total
	return prcnt
}

func saveImage(filename string, img image.Image) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, nil)
}
