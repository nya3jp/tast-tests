// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Contacts:     []string{"whalechang@chromium.org", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

func countOfVolumeChanges(ctx context.Context, kb *input.KeyboardEventWriter) (int, error) {
	numberOfVolumeUpChanges := 0
	numberOfVolumeDownChanges := 0

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	vh, err := audionode.NewVolumeHelper(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to create the volumeHelper")
	}
	originalVolume, err := vh.ActiveNodeVolume(ctx)

	defer func() {
		if err := vh.SetVolume(ctxForCleanUp, originalVolume); err != nil {
			testing.ContextLog(ctxForCleanUp, "Failed to SetVolume to original on defer: ", err)
		}
	}()

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

func verifyNumberOfVolumeChanges(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter, numberOfVolumeSteps, expectNumberOfVolumeChanges int) error {
	testing.ContextLog(ctx, "Setup fake USB playback numberOfVolumeSteps: ", numberOfVolumeSteps)
	unload, err := audio.LoadFakeUSBSoundcard(ctx, numberOfVolumeSteps)
	if err != nil {
		return errors.Wrap(err, "failed to load fake USB soundcard")
	}
	defer unload(ctx)
	if err := audio.SetupFakeUSBNode(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to setup fake USB node")
	}
	testing.ContextLog(ctx, "Testing volume steps: ", numberOfVolumeSteps)
	numberOfVolumeChanges, err := countOfVolumeChanges(ctx, kb)
	if err != nil {
		return errors.Wrap(err, "fail to perform volume controls up and down")
	}
	if numberOfVolumeChanges != expectNumberOfVolumeChanges {
		return errors.Errorf("test failure: expect numberOfVolumeChanges: %d == expectNumberOfVolumeChanges: %d", numberOfVolumeChanges, expectNumberOfVolumeChanges)
	}
	return nil
}

func USBVolumeGranularity(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}

	defer func() {
		if err := kb.Close(); err != nil {
			testing.ContextLog(ctxForCleanUp, "Failed close virtual keyboard: ", err)
		}
		if err := kb.Accel(ctxForCleanUp, "Ctrl+W"); err != nil {
			testing.ContextLog(ctxForCleanUp, "Failed Accel virtual keyboard: ", err)
		}
	}()

	// For case 0-9 steps, expectNumberOfVolumeChanges is 25
	expectNumberOfVolumeChanges := 25
	for numberOfVolumeSteps := 0; numberOfVolumeSteps <= 9; numberOfVolumeSteps++ {
		err := verifyNumberOfVolumeChanges(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
		if err != nil {
			s.Errorf("Fail to verify numberOfVolumeSteps[%d] expectNumberOfVolumeChanges[%d]: %v", numberOfVolumeSteps, expectNumberOfVolumeChanges, err)
		}
	}
	// For case 10-25 steps, numberOfVolumeSteps == expectNumberOfVolumeChanges
	for numberOfVolumeSteps := 10; numberOfVolumeSteps <= 25; numberOfVolumeSteps++ {
		expectNumberOfVolumeChanges := numberOfVolumeSteps
		err := verifyNumberOfVolumeChanges(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
		if err != nil {
			s.Errorf("Fail to verify numberOfVolumeSteps[%d] expectNumberOfVolumeChanges[%d]: %v", numberOfVolumeSteps, expectNumberOfVolumeChanges, err)
		}
	}
	// For case > 25 steps, expectNumberOfVolumeChanges is 25
	numberOfVolumeSteps := 26
	expectNumberOfVolumeChanges = 25
	err = verifyNumberOfVolumeChanges(ctx, cr, kb, numberOfVolumeSteps, expectNumberOfVolumeChanges)
	if err != nil {
		s.Errorf("Fail to verify numberOfVolumeSteps[%d] expectNumberOfVolumeChanges[%d]: %v", numberOfVolumeSteps, expectNumberOfVolumeChanges, err)
	}
}
