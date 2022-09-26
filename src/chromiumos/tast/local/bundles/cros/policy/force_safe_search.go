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
		Func:         ForceSafeSearch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test the behavior of deprecated ForceSafeSearch policy: check if Google and YouTube safe search is enabled based on the value of the policy",
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
		Timeout: 9 * time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ForceYouTubeRestrict{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.ForceSafeSearch{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.ForceYouTubeSafetyMode{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.ForceGoogleSafeSearch{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func ForceSafeSearch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		name            string
		wantGoogleSafe  bool
		wantYouTubeSafe bool
		value           []policy.Policy
	}{
		{
			name:            "enabled",
			wantGoogleSafe:  true,
			wantYouTubeSafe: true,
			value:           []policy.Policy{&policy.ForceSafeSearch{Val: true}},
		},
		{
			name: "enabled_overwritten_by_ForceGoogleSafeSearch",
			// ForceSafeSearch is ignored entirely if ForceGoogleSafeSearch is set.
			wantGoogleSafe:  false,
			wantYouTubeSafe: false,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: true},
				&policy.ForceGoogleSafeSearch{Val: false}},
		},
		{
			name: "enabled_overwritten_by_ForceYouTubeSafetyMode",
			// ForceSafeSearch is ignored entirely if ForceYouTubeSafetyMode is set.
			wantGoogleSafe:  false,
			wantYouTubeSafe: false,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: true},
				&policy.ForceYouTubeSafetyMode{Val: false}},
		},
		{
			name: "enabled_overwritten_by_ForceYouTubeRestrict",
			// ForceSafeSearch is ignored entirely if ForceYouTubeRestrict is set.
			wantGoogleSafe:  false,
			wantYouTubeSafe: false,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: true},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictDisabled}},
		},
		{
			name:            "disabled",
			wantGoogleSafe:  false,
			wantYouTubeSafe: false,
			value:           []policy.Policy{&policy.ForceSafeSearch{Val: false}},
		},
		{
			name:            "disabled_overwritten_by_ForceGoogleSafeSearch",
			wantGoogleSafe:  true,
			wantYouTubeSafe: false,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: false},
				&policy.ForceGoogleSafeSearch{Val: true}},
		},
		{
			name:            "disabled_overwritten_by_ForceYouTubeSafetyMode",
			wantGoogleSafe:  false,
			wantYouTubeSafe: true,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: false},
				&policy.ForceYouTubeSafetyMode{Val: true}},
		},
		{
			name:            "disabled_overwritten_by_ForceYouTubeRestrict",
			wantGoogleSafe:  false,
			wantYouTubeSafe: true,
			value: []policy.Policy{
				&policy.ForceSafeSearch{Val: false},
				&policy.ForceYouTubeRestrict{Val: safesearch.ForceYouTubeRestrictModerate}},
		},
		{
			name:            "unset",
			wantGoogleSafe:  false,
			wantYouTubeSafe: false,
			value:           []policy.Policy{&policy.ForceSafeSearch{Stat: policy.StatusUnset}},
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

			if err := safesearch.TestGoogleSafeSearch(ctx, br, param.wantGoogleSafe); err != nil {
				s.Error("Failed to verify state of Google safe search: ", err)
			}

			if err := safesearch.TestYouTubeRestrictedMode(ctx, br, param.wantYouTubeSafe, false); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}
