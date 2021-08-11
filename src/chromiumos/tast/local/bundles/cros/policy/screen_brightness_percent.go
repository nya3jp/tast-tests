// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ScreenBrightnessPercent,
		Desc: "Test behavior of ScreenBrightnessPercent policy: check if the screen brightness matches the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			ui := uiauto.New(tconn)

			// Find the Status tray node and click to open it.
			statusTray := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
			if err := uiauto.Combine("find and click the status tray",
				ui.WaitUntilExists(statusTray),
				ui.LeftClick(statusTray),
			)(ctx); err != nil {
				s.Fatal("Failed to find and click the status try: ", err)
			}

			defer func() {
				// Close the Status tray again, otherwise the next subtest won't find it.
				if err := ui.LeftClick(statusTray)(ctx); err != nil {
					s.Fatal("Failed to close Status tray: ", err)
				}
			}()

			// Get the NodeInfo of the Brightness slider.
			brightnessSlider := nodewith.Name("Brightness").Role(role.Slider)
			sliderInfo, err := ui.Info(ctx, brightnessSlider)
			if err != nil {
				s.Fatal("Failed to find Brightness slider: ", err)
			}

			if sliderInfo.Value != param.wantBrightness {
				s.Errorf("Unexpected brightness set: got %s; want %s", sliderInfo.Value, param.wantBrightness)
			}
		})
	}
}
