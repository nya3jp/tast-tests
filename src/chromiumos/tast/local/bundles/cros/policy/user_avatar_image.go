// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/imagehelpers"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserAvatarImage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of the UserAvatarImage policy when the PersonalizationHub flag is enabled: verify that the user cannot change the device account image when the policy is set, otherwise, the user can change it",
		Contacts: []string{
			"pzliu@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Data:         []string{"user_avatar_image.jpeg"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.UserAvatarImage{}, pci.VerifiedFunctionalityUI),
		},
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
	jpegBytes, err := imagehelpers.GetJPEGBytesFromFilePath(s.DataPath("user_avatar_image.jpeg"))
	if err != nil {
		s.Fatal("Failed to read user avatar image: ", err)
	}
	imgURL, imgHash := eds.ServePolicyData(jpegBytes)

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

			// Open the personalization hub.
			ui := uiauto.New(tconn)
			if err := uiauto.Combine("Click open avatar subpage button",
				personalization.OpenPersonalizationHub(ui),
				personalization.OpenAvatarSubpage(ui),
			)(ctx); err != nil {
				s.Fatal("Failed to click open avatar subpage button: ", err)
			}

			breadcrumbAvatar := personalization.BreadcrumbNodeFinder(personalization.AvatarSubpageName)

			if param.shouldMatchPolicyImage {
				avatarSubpageButton := nodewith.Name(personalization.ChangeAvatar).HasClass("tast-open-subpage")
				if err := ui.WithTimeout(time.Second).LeftClick(avatarSubpageButton)(ctx); err != nil {
					s.Fatal("Failed to continue clicking avatar subpage button: ", err)
				}
				if found, err := ui.IsNodeFound(ctx, breadcrumbAvatar); found != false {
					s.Fatal("Failed to verify that avatar subpage is disabled: ", err)
				}
			} else {
				if err := uiauto.Combine("Confirm that avatar subpage is open",
					ui.WaitUntilExists(breadcrumbAvatar),
					ui.LeftClick(breadcrumbAvatar),
				)(ctx); err != nil {
					s.Fatal("Failed to confirm that avatar subpage is open: ", err)
				}

				deviceAccountImages, err := ui.NodesInfo(ctx, nodewith.Role(role.ListBoxOption))
				if err != nil {
					s.Fatal("Failed to get deviceAccountImages for selector node: ", err)
				}

				// The list of avatars may change over time. Checking a fixed avatar ID may cause
				// regression test failure in the future. We address this issue by always choosing
				// the last avatar from the list.
				avatarImageNodeName := deviceAccountImages[len(deviceAccountImages)-1].Name
				if err := testDefaultUserAvatar(ctx, ui, avatarImageNodeName); err != nil {
					s.Fatalf("Failed to select default avatar - %v: %v", avatarImageNodeName, err)
				}
			}
		})
	}
}

func testDefaultUserAvatar(ctx context.Context, ui *uiauto.Context, imageName string) error {
	avatarOption := nodewith.Role(role.ListBoxOption).Name(imageName)
	selectedAvatar := nodewith.Role(role.Image).Name(imageName).Ancestor(nodewith.Name("Current Avatar"))

	if err := uiauto.Combine("select a default avatar and validate selected avatar",
		ui.FocusAndWait(avatarOption), // scroll down if necessary
		ui.WaitUntilExists(avatarOption),
		ui.LeftClick(avatarOption),
		ui.WaitUntilExists(selectedAvatar))(ctx); err != nil {
		return errors.Wrap(err, "failed to validate selected avatar")
	}
	return nil
}
