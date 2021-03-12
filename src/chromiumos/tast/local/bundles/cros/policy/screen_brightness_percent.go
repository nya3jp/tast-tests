// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ScreenBrightnessPercent,
		Desc: "Test behavior of ScreenBrightnessPercent policy: check if the screen brightness matches the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome", "display_backlight"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func ScreenBrightnessPercent(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		wantBrightness string                          // wantBrightness is the expected brightness value.
		policy         *policy.ScreenBrightnessPercent // policy is the policy we test.
	}{
		{
			name:           "brightness_1",
			wantBrightness: "83%",
			policy: &policy.ScreenBrightnessPercent{
				Val: &policy.ScreenBrightnessPercentValue{
					BrightnessAC:      83,
					BrightnessBattery: 83,
				},
			},
		},
		{
			name:           "brightness_2",
			wantBrightness: "47%",
			policy: &policy.ScreenBrightnessPercent{
				Val: &policy.ScreenBrightnessPercentValue{
					BrightnessAC:      47,
					BrightnessBattery: 47,
				},
			},
		},
		{
			name:           "unset",
			wantBrightness: "47%", // In the unset case the last set brightness is expected.
			policy:         &policy.ScreenBrightnessPercent{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Find the Status tray node and click to open it.
			paramsST := ui.FindParams{
				ClassName: "ash/StatusAreaWidgetDelegate",
			}
			nodeST, err := ui.FindWithTimeout(ctx, tconn, paramsST, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find Status tray: ", err)
			}
			defer nodeST.Release(ctx)

			if err := nodeST.LeftClick(ctx); err != nil {
				s.Fatal("Failed to open Status tray: ", err)
			}
			defer func() {
				// Close the Status tray again, otherwise the next subtest won't find it.
				if err := nodeST.LeftClick(ctx); err != nil {
					s.Fatal("Failed to close Status tray: ", err)
				}
			}()

			// Find the Brightness slider.
			paramsBS := ui.FindParams{
				Role: ui.RoleTypeSlider,
				Name: "Brightness",
			}
			nodeBS, err := ui.FindWithTimeout(ctx, tconn, paramsBS, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find Brightness slider: ", err)
			}
			defer nodeBS.Release(ctx)

			if nodeBS.Value != param.wantBrightness {
				s.Errorf("Unexpected brightness set: got %s; want %s", nodeBS.Value, param.wantBrightness)
			}

		})
	}
}
