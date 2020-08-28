// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsMicGainSlider,
		Desc: "Checks that the Quick Settings mic gain slider can be adjusted",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "audio_record"},
	})
}

// QuickSettingsMicGainSlider tests that the mic gain slider can be adjusted up and down.
func QuickSettingsMicGainSlider(ctx context.Context, s *testing.State) {
	// Enable the mic gain slider.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=system-tray-mic-gain"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

	initial, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed initial value check for mic gain slider: ", quicksettings.SliderTypeMicGain)
	}
	s.Log("Initial mic gain slider value: ", initial)

	decrease, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed to decrease mic gain slider: ", err)
	}
	s.Log("Decreased mic gain slider value: ", decrease)

	if decrease >= initial {
		s.Fatalf("Failed to decrease mic gain slider value; %v is not less than %v", decrease, initial)
	}

	increase, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed to increase mic gain slider: ", err)
	}
	s.Log("Increased mic gain slider value: ", increase)

	if increase <= decrease {
		s.Fatalf("Failed to increase mic gain slider value; %v is not greater than %v", increase, decrease)
	}
}
