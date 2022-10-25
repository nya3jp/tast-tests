// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBVolumeGranularity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check for USB device volume changes depending on the volume range reported by the USB device",
		Contacts:     []string{"whalechang@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

func performVolumeControlsUpAndDown(ctx context.Context, kb *input.KeyboardEventWriter) (int, error) {
	numberOfVolumeUpChanges := 0
	numberOfVolumeDownChanges := 0

	vh, err := audionode.NewVolumeHelper(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to create the volumeHelper")
	}
	originalVolume, err := vh.ActiveNodeVolume(ctx)
	defer vh.SetVolume(ctx, originalVolume)

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		return -1, errors.Wrap(err, "failed to obtain the top-row layout")
	}

	if err = kb.Accel(ctx, topRow.VolumeUp); err != nil {
		return -1, errors.Wrap(err, "failed to press 'VolumeUp'")
	}

	testing.ContextLog(ctx, "Press 'VolumeUp' to make sure unmute")

	audioVh, err := audio.NewVolumeHelper(ctx)
	muted, err := audioVh.IsMuted(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to check audio mute status after pressing volumeup")
	}

	if muted {
		return -1, errors.New("failed to unmute the audio")
	}

	testing.ContextLog(ctx, "Decrease volume to 0")
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return -1, errors.Wrap(err, "failed to get volume during volume decrease")
		}
		if volume == 0 {
			break
		}
		if err := vh.VerifyVolumeChanged(ctx, func() error {
			return kb.Accel(ctx, topRow.VolumeDown)
		}); err != nil {
			return -1, errors.Wrap(err, "failed to change volume after pressing 'VolumeDown'")
		}
	}

	testing.ContextLog(ctx, "Increase volume to 100 and count number of key press")
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return -1, errors.Wrap(err, "failed to get volume during volume increase")
		}
		if volume == 100 {
			break
		}

		if err := vh.VerifyVolumeChanged(ctx, func() error {
			numberOfVolumeUpChanges++
			return kb.Accel(ctx, topRow.VolumeUp)
		}); err != nil {
			return -1, errors.Wrap(err, "failed to change volume after pressing 'VolumeUp'")
		}

	}
	testing.ContextLog(ctx, "Decrease volume until mute and count number of key press")
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return -1, errors.Wrap(err, "failed to get volume during volume decrease")
		}
		if volume == 0 {
			break
		}

		if err := vh.VerifyVolumeChanged(ctx, func() error {
			numberOfVolumeDownChanges++
			return kb.Accel(ctx, topRow.VolumeDown)
		}); err != nil {
			return -1, errors.Wrap(err, "failed to change volume after pressing 'VolumeDown'")
		}

	}
	if numberOfVolumeUpChanges != numberOfVolumeDownChanges {
		return -1, errors.Wrapf(err, "incorrect behaviour,  numberOfVolumeUpChanges: %d, numberOfVolumeDownChanges: %d shoud be same", numberOfVolumeUpChanges, numberOfVolumeDownChanges)
	}
	return numberOfVolumeUpChanges, nil
}

func verifyNumberOfVolumeChangs(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter, numberOfVolumeSteps, expectNumberOfVolumeChanges int) error {
	testing.ContextLog(ctx, "Setup fake USB playback")
	unload, err := audio.LoadFakeUSBSoundcard(ctx, numberOfVolumeSteps)
	if err != nil {
		return errors.Wrap(err, "failed to load fake USB soundcard")
	}
	defer unload(ctx)
	if err := audio.SetupFakeUSBNode(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to setup fake USB soundcard")
	}
	testing.ContextLog(ctx, "Testing volume steps: ", numberOfVolumeSteps)
	numberOfVolumeChangs, err := performVolumeControlsUpAndDown(ctx, kb)
	if err != nil {
		return errors.Wrap(err, "Fail to perform volume controls up and down ")
	}
	if numberOfVolumeChangs != expectNumberOfVolumeChanges {
		return errors.Errorf("test failure: expect numberOfVolumeChangs: %d == expectNumberOfVolumeChanges: %d", numberOfVolumeChangs, expectNumberOfVolumeChanges)
	}
	return nil
}

func USBVolumeGranularity(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Close audio player window as cleanup.
	defer kb.Accel(ctx, "Ctrl+W")

	// For case 0-9 steps, expectNumberOfVolumeChanges is 25
	expectNumberOfVolumeChanges := 25
	for numberOfVolumeSteps := 0; numberOfVolumeSteps <= 9; numberOfVolumeSteps++ {
		err := verifyNumberOfVolumeChangs(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
		if err != nil {
			s.Fatal("Fail to verify number of volume changes: ", err)
		}
	}
	// For case 10-25 steps, numberOfVolumeSteps == expectNumberOfVolumeChanges
	for numberOfVolumeSteps := 10; numberOfVolumeSteps <= 25; numberOfVolumeSteps++ {
		expectNumberOfVolumeChanges := numberOfVolumeSteps
		err := verifyNumberOfVolumeChangs(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
		if err != nil {
			s.Error("Fail to verify number of volume changes: ", err)
		}
	}
	// For case > 25 steps, expectNumberOfVolumeChanges is 25
	numberOfVolumeSteps := 26
	expectNumberOfVolumeChanges = 25
	err = verifyNumberOfVolumeChangs(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
	if err != nil {
		s.Error("Fail to verify number of volume changes: ", err)
	}
}
