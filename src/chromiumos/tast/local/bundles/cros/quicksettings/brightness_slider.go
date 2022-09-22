// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrightnessSlider,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the Quick Settings brightness slider can be adjusted by keyboard",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"sylvieliu@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		HardwareDeps: hwdep.D(hwdep.Microphone(), hwdep.SkipOnModel("kakadu", "atlas")),
	})
}

// BrightnessSlider tests that the brightness slider can be adjusted up and down.
func BrightnessSlider(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set up the keyboard, which is used to increment/decrement the slider.
	// TODO(crbug/1123231): use better slider automation controls if possible, instead of keyboard controls.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	initialBrightness, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeBrightness)
	if err != nil {
		s.Fatal("Failed initial value check for brightness slider: ", err)
	}
	s.Log("Initial brightness slider value: ", initialBrightness)

	decreaseBrightness, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeBrightness)
	if err != nil {
		s.Fatal("Failed to decrease brightness slider: ", err)
	}
	s.Log("Decreased brightness slider value: ", decreaseBrightness)

	increaseBrightness, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeBrightness)
	if err != nil {
		s.Fatal("Failed to increase brightness slider: ", err)
	}
	s.Log("Increased brightness slider value: ", increaseBrightness)
}
