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

	var (
		systemTrayFindParams = ui.FindParams{
			ClassName: "SystemTrayContainer",
		}
		systemTrayButtonFindParams = ui.FindParams{
			Role:      ui.RoleTypeButton,
			ClassName: "UnifiedSystemTray",
		}
		mutedFindParams = ui.FindParams{
			Name: "Toggle Volume. Volume is muted.",
			Role: ui.RoleTypeToggleButton,
		}
		unmutedFindParams = ui.FindParams{
			Name: "Toggle Volume. Volume is on, toggling will mute audio.",
			Role: ui.RoleTypeToggleButton,
		}
	)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AudioOutputAllowed
		// expectedMuted contains the expected state of the device.
		expectedMuted bool
	}{
		{
			name:          "true",
			value:         &policy.AudioOutputAllowed{Val: true},
			expectedMuted: false,
		},
		{
			name:          "false",
			value:         &policy.AudioOutputAllowed{Val: false},
			expectedMuted: true,
		},
		{
			name:          "unset",
			value:         &policy.AudioOutputAllowed{Stat: policy.StatusUnset},
			expectedMuted: false,
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

			// Toggle the system tray to update after the policy change.
			if err := ui.StableFindAndClick(ctx, tconn, systemTrayButtonFindParams, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to toggle system tray: ", err)
			}
			if err := ui.StableFindAndClick(ctx, tconn, systemTrayButtonFindParams, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to toggle system tray: ", err)
			}

			// System tray should be shown, click again if that is not the case.
			if exist, err := ui.Exists(ctx, tconn, systemTrayFindParams); err != nil {
				s.Fatal("Failed to check the state of the system tray")
			} else if !exist {
				if err := ui.StableFindAndClick(ctx, tconn, systemTrayButtonFindParams, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
					s.Fatal("Failed to toggle system tray: ", err)
				}
			}

			// Check if device is not muted.
			if err := ui.WaitUntilExistsStatus(ctx, tconn, unmutedFindParams, !param.expectedMuted, 5*time.Second); err != nil {
				s.Error("Could not confirm the device is not muted: ", err)
			}

			// Check if device is muted.
			if err := ui.WaitUntilExistsStatus(ctx, tconn, mutedFindParams, param.expectedMuted, 1*time.Second); err != nil {
				s.Error("Could not confirm the device is muted: ", err)
			}

			// Check if we can unmute.
			if param.expectedMuted {
				if err := ui.StableFindAndClick(ctx, tconn, mutedFindParams, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
					s.Fatal("Failed to click the audio toggle: ", err)
				}

				if err := testing.Sleep(ctx, 1*time.Second); err != nil {
					s.Fatal("Failed to sleep: ", err)
				}

				// Check if device is still muted.
				if err := ui.WaitUntilExistsStatus(ctx, tconn, unmutedFindParams, false, 5*time.Second); err != nil {
					s.Error("Could not confirm the device is not muted: ", err)
				}
				if err := ui.WaitUntilExistsStatus(ctx, tconn, mutedFindParams, true, 5*time.Second); err != nil {
					s.Error("Could not confirm the device is muted: ", err)
				}
			}

		})
	}
}
