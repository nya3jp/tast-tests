// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// crasTestClientPattern parses the output of cras_test_client.
var crasTestClientPattern = regexp.MustCompile(strings.Join([]string{
	`^System Volume \(0-100\): (\d+) (\(Muted\))?`,
	`Capture Gain \(-?\d+\.\d+ - -?\d+\.\d+\): (-?\d+\.\d+)dB `,
	``}, "\n"))

type audioSettings struct {
	volume uint
	gain   float64
	muted  bool
}

// readAudioSettings reads the volume, recorder gain in decibels, and system
// mute state.
func readAudioSettings(ctx context.Context) (*audioSettings, error) {
	output, err := testexec.CommandContext(ctx, "cras_test_client").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "unable to call cras_test_client")
	}
	match := crasTestClientPattern.FindSubmatch(output)
	if match == nil {
		return nil, errors.Wrapf(err, "unable to parse audio settings from output %q", output)
	}
	volume, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse volume from %q", match[1])
	}
	muted := match[2] != nil
	gain, err := strconv.ParseFloat(string(match[3]), 64)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse gain from %q", match[3])
	}
	return &audioSettings{
		volume: uint(volume),
		gain:   gain,
		muted:  muted,
	}, nil
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

func setAudioVolume(ctx context.Context, volume uint) error {
	volumeArg := strconv.FormatUint(uint64(volume), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--volume", volumeArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio volume")
	}
	return nil
}

func setupAudioGain(ctx context.Context, gain float64, prev *audioSettings) Result {
	if err := setAudioGain(ctx, gain); err != nil {
		return ResultFailed(err)
	}
	testing.ContextLogf(ctx, "Set audio gain to %f from %f", gain, prev.gain)

	return ResultSucceeded(func(ctx context.Context) error {
		if err := setAudioGain(ctx, prev.gain); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset audio gain to %f", prev.gain)
		return nil
	})
}

func setupAudioMuted(ctx context.Context, muted bool, prev *audioSettings) Result {
	if err := setAudioMuted(ctx, muted); err != nil {
		return ResultFailed(err)
	}
	testing.ContextLogf(ctx, "Set muted to %t from %t", muted, prev.muted)

	return ResultSucceeded(func(ctx context.Context) error {
		if err := setAudioMuted(ctx, prev.muted); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset muted to %t", prev.muted)
		return nil
	})
}

func setupAudioVolume(ctx context.Context, volume uint, prev *audioSettings) Result {
	if err := setAudioVolume(ctx, volume); err != nil {
		return ResultFailed(err)
	}
	testing.ContextLogf(ctx, "Set volume to %d from %d", volume, prev.volume)

	return ResultSucceeded(func(ctx context.Context) error {
		if err := setAudioVolume(ctx, prev.volume); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset volume to %d", prev.volume)
		return nil
	})
}

// MuteAudio sets volume to zero, recording, gain to zero, and mutes audio.
func MuteAudio(ctx context.Context) Result {
	return ResultNested(ctx, func(s *Setup) error {
		prev, err := readAudioSettings(ctx)
		if err != nil {
			return err
		}
		s.Add(setupAudioGain(ctx, 0.0, prev))
		s.Add(setupAudioMuted(ctx, true, prev))
		s.Add(setupAudioVolume(ctx, 0, prev))
		return nil
	})
}
