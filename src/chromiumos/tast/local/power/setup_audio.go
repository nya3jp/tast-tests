// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// crasTestClientRe parses the output of cras_test_client.
var crasTestClientRe = regexp.MustCompile("^System Volume \\(0-100\\): (\\d+) (\\(Muted\\))?\nCapture Gain \\(-?\\d+\\.\\d+ - -?\\d+\\.\\d+\\): (-?\\d+\\.\\d+)dB \n")

// readAudioSettings reads the volume, recorder gain in decibels, and system
// mute state.
func readAudioSettings(ctx context.Context) (uint, float64, bool, error) {
	output, err := testexec.CommandContext(ctx, "cras_test_client").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrap(err, "unable to call cras_test_client")
	}
	match := crasTestClientRe.FindSubmatch(output)
	if match == nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse audio settings from output %q", output)
	}
	volume, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse volume from %q", match[1])
	}
	muted := match[2] != nil
	gain, err := strconv.ParseFloat(string(match[3]), 64)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse gain from %q", match[3])
	}
	return uint(volume), gain, muted, nil
}

func setAudioVolume(ctx context.Context, volume uint) error {
	volumeArg := strconv.FormatUint(uint64(volume), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--volume", volumeArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio volume")
	}
	return nil
}

// SetAudioVolume sets the audio output volume.
func SetAudioVolume(ctx context.Context, volume uint, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	prevVolume, _, _, err := readAudioSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := setAudioVolume(ctx, volume); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Set volume to %d from %d", volume, prevVolume)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := setAudioVolume(ctx, prevVolume); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Restored volume to %d", prevVolume)
		return nil
	})
}

func setAudioMuted(ctx context.Context, muted bool) error {
	mutedArg := "0"
	if muted {
		mutedArg = "1"
	}
	if err := testexec.CommandContext(ctx, "cras_test_client", "--mute", mutedArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio mute")
	}
	return nil
}

// SetAudioMuted sets the system mute.
func SetAudioMuted(ctx context.Context, muted bool, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	_, _, prevMuted, err := readAudioSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := setAudioMuted(ctx, muted); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Set muted to %t from %t", muted, prevMuted)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := setAudioMuted(ctx, prevMuted); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Set muted to %t", prevMuted)
		return nil
	})
}

func setAudioGain(ctx context.Context, gain float64) error {
	// The --capture_gain argument takes a value in millibel
	const milliInDeci = 100
	gainArg := strconv.FormatInt(int64(gain*milliInDeci), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--capture_gain", gainArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set capture gain")
	}
	return nil
}

// SetAudioGain sets the audio recorder gain in decibels.
func SetAudioGain(ctx context.Context, gain float64, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	_, prevGain, _, err := readAudioSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := setAudioGain(ctx, gain); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Set audio gain to %f from %f", gain, prevGain)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := setAudioGain(ctx, prevGain); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset audio gain to %f", prevGain)
		return nil
	})
}

// MuteAudio sets volume to zero, recording, gain to zero, and mutes audio.
func MuteAudio(ctx context.Context, chain CleanupChain) (CleanupChain, error) {
	chain, err := SetAudioGain(ctx, 0.0, chain)
	if err != nil {
		return nil, err
	}
	chain, err = SetAudioMuted(ctx, true, chain)
	if err != nil {
		return nil, err
	}
	chain, err = SetAudioVolume(ctx, 0, chain)
	if err != nil {
		return nil, err
	}
	return chain, nil
}
