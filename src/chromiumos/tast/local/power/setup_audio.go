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

// setAudioVolume sets the system audio volume level.
func setAudioVolume(ctx context.Context, volume uint) error {
	volumeArg := strconv.FormatUint(uint64(volume), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--volume", volumeArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio volume")
	}
	return nil
}

// setAudioMuted enables or disables the system mute.
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

// setAudioGain sets the audio recorder gain in decibels.
func setAudioGain(ctx context.Context, gain float64) error {
	// The --capture_gain argument takes a value in millibel
	const milliInDeci = 100
	gainArg := strconv.FormatInt(int64(gain*milliInDeci), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--capture_gain", gainArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set capture gain")
	}
	return nil
}

// muteAudio is an Action that mutes audio.
type muteAudio struct {
	ctx        context.Context
	prevVolume uint
	prevGain   float64
	prevMuted  bool
}

// Setup sets volume to zero, mutes the audio, and sets the recording gain to
// zero.
func (a *muteAudio) Setup() error {
	prevVolume, prevGain, prevMuted, err := readAudioSettings(a.ctx)
	if err != nil {
		return err
	}
	a.prevVolume = prevVolume
	a.prevGain = prevGain
	a.prevMuted = prevMuted
	if err := setAudioVolume(a.ctx, 0); err != nil {
		return err
	}
	if err := setAudioMuted(a.ctx, true); err != nil {
		a.Cleanup()
		return err
	}
	if err := setAudioGain(a.ctx, 0.0); err != nil {
		a.Cleanup()
		return err
	}
	return nil
}

// Cleanup restores the mute state, volume, and gain.
func (a *muteAudio) Cleanup() error {
	var result error
	if err := setAudioVolume(a.ctx, a.prevVolume); err != nil {
		result = err
	}
	if err := setAudioMuted(a.ctx, a.prevMuted); err != nil {
		result = err
	}
	if err := setAudioGain(a.ctx, a.prevGain); err != nil {
		result = err
	}
	return result
}

// MuteAudio creates an Action that sets volume to zero, recording
// gain to zero, and mutes audio.
func MuteAudio(ctx context.Context) Action {
	return &muteAudio{
		ctx:        ctx,
		prevGain:   0.0,
		prevMuted:  false,
		prevVolume: 0,
	}
}
