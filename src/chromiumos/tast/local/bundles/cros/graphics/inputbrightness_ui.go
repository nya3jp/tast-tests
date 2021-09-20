// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/graphics/brightness"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputbrightnessUI,
		Desc:         "Verifies system Brightness increase and decrease through UI",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func InputbrightnessUI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		wantBrightness string
		policy         *policy.ScreenBrightnessPercent
	}{
		{
			name:           "brightnessDecreaseCheck",
			wantBrightness: "0%",
			policy: &policy.ScreenBrightnessPercent{
				Val: &policy.ScreenBrightnessPercentValue{
					BrightnessAC:      0,
					BrightnessBattery: 0,
				},
			},
		},
		{
			name:           "brightnessIncreaseCheck",
			wantBrightness: "100%",
			policy: &policy.ScreenBrightnessPercent{
				Val: &policy.ScreenBrightnessPercentValue{
					BrightnessAC:      100,
					BrightnessBattery: 100,
				},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
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
			statusTrayToggler := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
			if err := uiauto.Combine("find and click the status tray",
				ui.LeftClick(statusTrayToggler),
			)(ctx); err != nil {
				s.Fatal("Failed to find and click the status tray: ", err)
			}

			defer func() {
				// Close the Status tray again, otherwise the next subtest won't find it.
				if err := ui.LeftClick(statusTrayToggler)(ctx); err != nil {
					s.Fatal("Failed to click to close Status tray: ", err)
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
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				brightness, err := brightness.SystemBrightness(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get system brightness"))
				}
				expBrightness, err := strconv.ParseFloat(strings.Trim(param.wantBrightness, "%"), 64)
				if err != nil {
					return errors.Wrap(err, "failed to convert string to floating value")
				}
				if brightness != expBrightness {
					return errors.Errorf("expected brightness %q but got %q", expBrightness, brightness)
				}
				return nil
			}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
				s.Fatal("Failed to decrease brightness: ", err)
			}
		})
	}
}
