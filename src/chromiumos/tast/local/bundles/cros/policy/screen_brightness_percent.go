// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenBrightnessPercent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test behavior of ScreenBrightnessPercent policy: check if the screen brightness matches the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		// no_qemu: VMs don't support brightness control.
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ScreenBrightnessPercent{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func ScreenBrightnessPercent(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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

			// Find the Status unifided system tray node and click to open it.
			statusTray := nodewith.ClassName("UnifiedSystemTray")
			if err := uiauto.Combine("find and click the status area unified system tray",
				ui.WaitUntilExists(statusTray),
				ui.LeftClick(statusTray),
			)(ctx); err != nil {
				s.Fatal("Failed to find and click the status area unified system tray: ", err)
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
