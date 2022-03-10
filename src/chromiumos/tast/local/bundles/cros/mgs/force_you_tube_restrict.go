// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil/safesearch"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceYouTubeRestrict,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify behavior of ForceYouTubeRestrict policy on Managed Guest Session",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
		// Loading two YouTube videos on slower devices can take a while (we observed subtests that took up to 40 seconds), thus give every subtest 1 minute to run.
		Timeout: 4 * time.Minute,
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ForceYouTubeRestrict
		// stringContentRestricted is whether strong content is expected to be restricted.
		strongContentRestricted bool
		// mildContentRestricted is whether mild content is expected to be restricted.
		mildContentRestricted bool
	}{
		{
			name:                    "disabled",
			value:                   &policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictDisabled},
			strongContentRestricted: false,
			mildContentRestricted:   false,
		},
		{
			name:                    "moderate",
			value:                   &policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictModerate},
			strongContentRestricted: true,
			mildContentRestricted:   false,
		},
		{
			name:                    "strict",
			value:                   &policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictStrict},
			strongContentRestricted: true,
			mildContentRestricted:   true,
		},
		{
			name:                    "unset",
			value:                   &policy.ForceYouTubeRestrict{Stat: policy.StatusUnset},
			strongContentRestricted: false,
			mildContentRestricted:   false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Launch a new MGS with default account.
			mgs, cr, err := mgs.New(
				ctx,
				fdms,
				mgs.DefaultAccount(),
				mgs.AutoLaunch(mgs.MgsAccountID),
				mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{param.value}),
			)
			if err != nil {
				s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
			}
			defer func() {
				if err := mgs.Close(ctx); err != nil {
					s.Fatal("Failed close MGS: ", err)
				}
			}()
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_")
			br := cr.Browser()

			// Run actual test.
			if err := safesearch.TestYouTubeRestrictedMode(ctx, br, param.strongContentRestricted, param.mildContentRestricted); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}
