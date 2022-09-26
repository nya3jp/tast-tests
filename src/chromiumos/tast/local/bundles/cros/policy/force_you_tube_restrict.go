// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/safesearch"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceYouTubeRestrict,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check if YouTube content restrictions work as specified by the ForceYouTubeRestrict policy",
		Contacts: []string{
			"sinhak@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		// Loading two YouTube videos on slower devices can take a while (we observed subtests that took up to 40 seconds), thus give every subtest 1 minute to run.
		Timeout: 4 * time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ForceYouTubeRestrict{}, pci.VerifiedFunctionalityJS),
		},
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Run actual test.
			if err := safesearch.TestYouTubeRestrictedMode(ctx, br, param.strongContentRestricted, param.mildContentRestricted); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}
