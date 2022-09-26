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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioOutputAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check if AudioOutputAllowed forces the device to be muted",
		Contacts: []string{
			"vsavu@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AudioOutputAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func AudioOutputAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Quicksettings should be hidden.
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to hide Quicksettings: ", err)
	}

	mutedFinder := nodewith.Name("Toggle Volume. Volume is muted.").Role(role.ToggleButton)

	unmutedFinder := nodewith.Name("Toggle Volume. Volume is on, toggling will mute audio.").Role(role.ToggleButton)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AudioOutputAllowed
		// expectedElement contains search parameters for the audio element.
		expectedElement *nodewith.Finder
		// expectDisabled checks if the output button is disabled.
		expectDisabled bool
	}{
		{
			name:            "true",
			value:           &policy.AudioOutputAllowed{Val: true},
			expectedElement: unmutedFinder,
		},
		{
			name:            "false",
			value:           &policy.AudioOutputAllowed{Val: false},
			expectedElement: mutedFinder,
			expectDisabled:  true,
		},
		{
			name:            "unset",
			value:           &policy.AudioOutputAllowed{Stat: policy.StatusUnset},
			expectedElement: unmutedFinder,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Show Quicksettings.
			if err := quicksettings.ShowWithRetry(ctx, tconn, 5*time.Second); err != nil {
				s.Fatal("Failed to show Quicksettings : ", err)
			}
			defer quicksettings.Hide(ctx, tconn)

			// Check if device is not muted.
			ui := uiauto.New(tconn)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(param.expectedElement)(ctx); err != nil {
				s.Fatal("Audio output invalid state: ", err)
			}

			// Check if we can unmute with disabled audio.
			if param.expectDisabled {
				if err := ui.WithTimeout(1 * time.Second).LeftClick(mutedFinder)(ctx); err != nil {
					s.Fatal("Failed to click the audio toggle: ", err)
				}

				// Check if device is still muted.
				if err := policyutil.VerifyNotExists(ctx, tconn, unmutedFinder, 2*time.Second); err != nil {
					s.Error("Could not confirm the device is muted: ", err)
				}
			}

		})
	}
}
