// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AudioOutputAllowed,
		Desc: "Check if AudioOutputAllowed forces the device to be muted",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func AudioOutputAllowed(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Quicksettings should be hidden.
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to hide Quicksettings: ", err)
	}

	mutedFindParams := ui.FindParams{
		Name: "Toggle Volume. Volume is muted.",
		Role: ui.RoleTypeToggleButton,
	}
	unmutedFindParams := ui.FindParams{
		Name: "Toggle Volume. Volume is on, toggling will mute audio.",
		Role: ui.RoleTypeToggleButton,
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AudioOutputAllowed
		// expectedElement contains search parameters for the audio element.
		expectedElement ui.FindParams
		// expectDisabled checks if the output button is disabled.
		expectDisabled bool
	}{
		{
			name:            "true",
			value:           &policy.AudioOutputAllowed{Val: true},
			expectedElement: unmutedFindParams,
		},
		{
			name:            "false",
			value:           &policy.AudioOutputAllowed{Val: false},
			expectedElement: mutedFindParams,
			expectDisabled:  true,
		},
		{
			name:            "unset",
			value:           &policy.AudioOutputAllowed{Stat: policy.StatusUnset},
			expectedElement: unmutedFindParams,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

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
			if err := ui.WaitUntilExists(ctx, tconn, param.expectedElement, 5*time.Second); err != nil {
				s.Fatal("Audio output invalid state: ", err)
			}

			// Check if we can unmute with disabled audio.
			if param.expectDisabled {
				if err := ui.StableFindAndClick(ctx, tconn, mutedFindParams, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
					s.Fatal("Failed to click the audio toggle: ", err)
				}

				// Check if device is still muted.
				if err := policyutil.VerifyNotExists(ctx, tconn, unmutedFindParams, 2*time.Second); err != nil {
					s.Error("Could not confirm the device is muted: ", err)
				}
			}

		})
	}
}
