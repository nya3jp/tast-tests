// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj/volume"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	// expectedBrightness indicates the default screen brightness.
	expectedBrightness = 80
	// expectedVolumePercent indicates the percentage of maximum volume.
	expectedVolumePercent = 10
)

// SetUp sets all initial settings to DUT before performing CUJ testing.
func SetUp(ctx context.Context) (setup.CleanupCallback, error) {
	setBrightnessNormal, err := SetScreenBrightness(ctx, expectedBrightness)
	if err != nil {
		return func(context.Context) error { return nil }, err
	}

	setVolumeNormal, err := SetAudioVolume(ctx, expectedVolumePercent)
	if err != nil {
		return func(context.Context) error { return nil }, err
	}

	return func(ctx context.Context) error {
		setBrightnessErr := setBrightnessNormal(ctx)
		setVolumeErr := setVolumeNormal(ctx)
		if setBrightnessErr != nil {
			return errors.Wrap(setBrightnessErr, "failed to reset brightness setting")
		}
		if setVolumeErr != nil {
			return errors.Wrap(setVolumeErr, "failed to reset volume setting")
		}
		return nil
	}, nil
}

// SetScreenBrightness sets the screen brightness to the expectedBrightness and returns a function that restores the original brightness.
func SetScreenBrightness(ctx context.Context, expectedBrightness float64) (setup.CleanupCallback, error) {
	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a PowerManager object")
	}
	originalBrightness, err := pm.GetScreenBrightnessPercent(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the original brightness")
	}
	testing.ContextLogf(ctx, "Setting brightness to default. Current brightness: %.2f%%", originalBrightness)
	if err := pm.SetScreenBrightness(ctx, expectedBrightness); err != nil {
		return nil, errors.Wrap(err, "failed to set the screen brightness")
	}
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting screen brightness to original brightness: %.2f%%", originalBrightness)
		return pm.SetScreenBrightness(ctx, originalBrightness)
	}, nil
}

// SetAudioVolume sets the audio volume to the expected percentage of the maximum volume and returns a function that restores the original volume.
func SetAudioVolume(ctx context.Context, expectedVolumePercent int) (setup.CleanupCallback, error) {
	vh, err := volume.NewVolumeHelper(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a volumeHelper")
	}
	originalVolume, err := vh.GetVolume(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get volume")
	}
	testing.ContextLogf(ctx, "Setting the audio volume to %d%% of the maximum volume. Current volume: %d", expectedVolumePercent, originalVolume)
	if err := vh.SetVolume(ctx, expectedVolumePercent); err != nil {
		return nil, errors.Wrap(err, "failed to set volume")
	}
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting volume to original volume: %d", originalVolume)
		return vh.SetVolume(ctx, originalVolume)
	}, nil
}
