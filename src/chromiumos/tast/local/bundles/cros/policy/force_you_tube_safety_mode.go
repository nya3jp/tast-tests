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
		Func:         ForceYouTubeSafetyMode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test the behavior of deprecated ForceYouTubeSafetyMode policy: check if YouTube safe search is enabled based on the value of the policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
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
		Timeout: 7 * time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ForceYouTubeSafetyMode{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.ForceYouTubeRestrict{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func ForceYouTubeSafetyMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		name                    string
		strongContentRestricted bool
		mildContentRestricted   bool
		value                   []policy.Policy
	}{
		{
			name:                    "enabled",
			strongContentRestricted: true,
			mildContentRestricted:   false,
			value:                   []policy.Policy{&policy.ForceYouTubeSafetyMode{Val: true}},
		},
		{
			name:                    "enabled_overwritten_by_ForceYouTubeRestrict_disabled",
			strongContentRestricted: false,
			mildContentRestricted:   false,
			value: []policy.Policy{
				&policy.ForceYouTubeSafetyMode{Val: true},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictDisabled}},
		},
		{
			name:                    "enabled_overwritten_by_ForceYouTubeRestrict_strict",
			strongContentRestricted: true,
			mildContentRestricted:   true,
			value: []policy.Policy{
				&policy.ForceYouTubeSafetyMode{Val: true},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictStrict}},
		},
		{
			name:                    "disabled",
			strongContentRestricted: false,
			mildContentRestricted:   false,
			value:                   []policy.Policy{&policy.ForceYouTubeSafetyMode{Val: false}},
		},
		{
			name:                    "disabled_overwritten_by_ForceYouTubeRestrict_moderate",
			strongContentRestricted: true,
			mildContentRestricted:   false,
			value: []policy.Policy{
				&policy.ForceYouTubeSafetyMode{Val: false},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictModerate}},
		},
		{
			name:                    "disabled_overwritten_by_ForceYouTubeRestrict_strict",
			strongContentRestricted: true,
			mildContentRestricted:   true,
			value: []policy.Policy{
				&policy.ForceYouTubeSafetyMode{Val: false},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictStrict}},
		},
		{
			name:                    "unset",
			strongContentRestricted: false,
			mildContentRestricted:   false,
			value:                   []policy.Policy{&policy.ForceYouTubeSafetyMode{Stat: policy.StatusUnset}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := safesearch.TestYouTubeRestrictedMode(ctx, br, param.strongContentRestricted, param.mildContentRestricted); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}
