// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// crasTestClientRE parses the output of cras_test_client.
var crasTestClientRE = regexp.MustCompile(`^System Volume \(0-100\): (\d+) (\(Muted\))?
Capture Muted : (Not muted|Muted)`)

type audioSettings struct {
	volume              uint
	muted, captureMuted bool
}

// readAudioSettings reads the volume, recorder gain in decibels, and system
// mute state.
func readAudioSettings(ctx context.Context) (*audioSettings, error) {
	output, err := testexec.CommandContext(ctx, "cras_test_client").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "unable to call cras_test_client")
	}
	match := crasTestClientRE.FindSubmatch(output)
	if match == nil {
		return nil, errors.Wrapf(err, "unable to parse audio settings from output %q", output)
	}
	volume, err := strconv.ParseUint(string(match[1]), 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse volume from %q", match[1])
	}
	muted := match[2] != nil
	captureMuted := string(match[3]) == "Muted"
	return &audioSettings{
		volume:       uint(volume),
		muted:        muted,
		captureMuted: captureMuted,
	}, nil
}

type crasMuteArg string

const (
	crasMute        crasMuteArg = "--mute"
	crasMuteCapture crasMuteArg = "--capture_mute"
)

func setAudioMuted(ctx context.Context, arg crasMuteArg, muted bool) error {
	mutedArg := "0"
	if muted {
		mutedArg = "1"
	}
	if err := testexec.CommandContext(ctx, "cras_test_client", string(arg), mutedArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to run cras_test_client %s %s", arg, mutedArg)
	}
	return nil
}

func setAudioVolume(ctx context.Context, volume uint) error {
	volumeArg := strconv.FormatUint(uint64(volume), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--volume", volumeArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to run cras_test_client --volume %s", volumeArg)
	}
	return nil
}

func setupAudioMuted(ctx context.Context, arg crasMuteArg, muted, prev bool) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "Setting %s to %t from %t", arg, muted, prev)
	if err := setAudioMuted(ctx, arg, muted); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting %s to %t", arg, prev)
		return setAudioMuted(ctx, arg, prev)
	}, nil
}

func setupAudioVolume(ctx context.Context, volume, prev uint) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "Setting volume to %d from %d", volume, prev)
	if err := setAudioVolume(ctx, volume); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting volume to %d", prev)
		return setAudioVolume(ctx, prev)
	}, nil
}

// MuteAudio sets volume to zero, recording, gain to zero, and mutes audio.
func MuteAudio(ctx context.Context) (CleanupCallback, error) {
	return Nested(ctx, "mute audio", func(s *Setup) error {
		prev, err := readAudioSettings(ctx)
		if err != nil {
			return err
		}
		s.Add(setupAudioMuted(ctx, crasMute, true, prev.muted))
		s.Add(setupAudioMuted(ctx, crasMuteCapture, true, prev.captureMuted))
		s.Add(setupAudioVolume(ctx, 0, prev.volume))
		return nil
	})
}
