// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VolumeSlider,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the Quick Settings volume slider can be adjusted by keyboard",
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

func muteUnmuteVolume(ctx context.Context, tconn *chrome.TestConn, vh *audio.Helper) error {
	mutedButton := nodewith.Name("Toggle Volume. Volume is muted.").Role(role.ToggleButton)
	unmutedButton := nodewith.Name("Toggle Volume. Volume is on, toggling will mute audio.").Role(role.ToggleButton)

	ui := uiauto.New(tconn)

	// Mute the volume.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(unmutedButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the volume toggle")
	}

	mute, err := vh.IsMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check volume mute status")
	}
	if !mute {
		return errors.New("failed to mute the volume")
	}

	// Unmute the volume.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(mutedButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the volume toggle")
	}

	mute, err = vh.IsMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check volume mute status")
	}
	if mute {
		return errors.New("failed to unmute the volume")
	}
	return nil
}

// VolumeSlider tests that the volume slider can be adjusted up and down.
func VolumeSlider(ctx context.Context, s *testing.State) {
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

	// Test the volume slider.
	initialVolume, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeVolume)
	if err != nil {
		s.Fatal("Failed initial value check for volume slider: ", err)
	}
	s.Log("Initial volume slider value: ", initialVolume)

	decreaseVolume, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeVolume)
	if err != nil {
		s.Fatal("Failed to decrease volume slider: ", err)
	}
	s.Log("Decreased volume slider value: ", decreaseVolume)

	increaseVolume, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeVolume)
	if err != nil {
		s.Fatal("Failed to increase volume slider: ", err)
	}
	s.Log("Increased volume slider value: ", increaseVolume)

	// Test the volume toggle.
	vh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	if err := muteUnmuteVolume(ctx, tconn, vh); err != nil {
		s.Fatal("Failed to mute/unmute volume: ", err)
	}
}
