// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/input"
)

// AudioFromFile inputs an audio file via active input node and waits for its completion.
func AudioFromFile(ctx context.Context, audioFilePath string) error {
	audioInput := audio.TestRawData{
		Path:          audioFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
	}

	// Playback function by CRAS.
	playCmd := crastestclient.PlaybackFileCommand(
		ctx, audioInput.Path,
		audioInput.Duration,
		audioInput.Channels,
		audioInput.Rate)
	playCmd.Start()

	return playCmd.Wait()
}

// EnableAloop loads and enables Aloop then sets it as active input/output node.
func EnableAloop(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context), error) {
	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load ALSA loopback module")
	}

	// Activate the Aloop nodes.
	if err := activateAloopNodes(ctx, tconn); err != nil {
		// Unload ALSA loopback if any following setups failed.
		unload(ctx)
		return nil, err
	}

	return unload, nil
}

func activateAloopNodes(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard: ")
	}
	defer kb.Close()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show Quick Settings: ")
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.SelectAudioOption(ctx, tconn, kb, "Loopback Playback"); err != nil {
		return errors.Wrap(err, "failed to select ALSA loopback output: ")
	}

	if err := quicksettings.SelectAudioOption(ctx, tconn, kb, "Loopback Capture"); err != nil {
		return errors.Wrap(err, "failed to select ALSA loopback input: ")
	}

	return nil
}
