// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
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
		Desc:         "Behavior of UserAvatarImage policy: verify that the user cannot change the device account image when the policy is set, otherwise, the user can change it",
		Contacts: []string{
			"mgawad@google.com", // Test author
			"pzliu@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Data:         []string{"user_avatar_image.jpeg"},
	})
}

func UserAvatarImage(ctx context.Context, s *testing.State) {
	const (
		firstImageName  = "Person daydreaming"
		firstImageID    = "84"
		secondImageName = "Basketball"
		secondImageID   = "53"
	)

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
	if err != nil {
		s.Fatal("Failed to read user avatar image: ", err)
	}

	for _, param := range []struct {
		name                   string
		shouldMatchPolicyImage bool                    // shouldMatchPolicyImage is a flag to check the image pixels.
		value                  *policy.UserAvatarImage // value is the value of the policy.
	}{
		{
			name:                   "non_empty",
			shouldMatchPolicyImage: true,
			value:                  &policy.UserAvatarImage{Stat: policy.StatusUnset},
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

				if err := testDefaultUserAvatar(ctx, ui, firstImageName, firstImageID); err != nil {
					s.Fatalf("Failed to select default avatar - %v: %v", firstImageName, err)
				}

				if err := testDefaultUserAvatar(ctx, ui, secondImageName, secondImageID); err != nil {
					s.Fatalf("Failed to select default avatar - %v: %v", secondImageName, err)
				}
			}
		})
	}
}

func testDefaultUserAvatar(ctx context.Context, ui *uiauto.Context, imageName, imageID string) error {
	avatarOption := nodewith.Role(role.ListBoxOption).Name(imageName)
	selectedAvatar := nodewith.HasClass(fmt.Sprintf("selected-default-user-image-%v", imageID)).NameContaining(imageName)

	if err := uiauto.Combine("select a default avatar and validate selected avatar",
		ui.WaitUntilExists(avatarOption),
		ui.LeftClick(avatarOption),
		ui.WaitUntilExists(selectedAvatar))(ctx); err != nil {
		return errors.Wrap(err, "failed to validate selected avatar")
	}
	return nil
}
