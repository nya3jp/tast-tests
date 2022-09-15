// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"fmt"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

// expectedVolumePercent indicates the percentage of maximum volume.
const expectedVolumePercent = 10

// InitializeSetting sets all initial settings to DUT before performing CUJ testing.
func InitializeSetting(ctx context.Context, tconn *chrome.TestConn) (action.Action, error) {
	setVolumeNormal, err := SetAudioVolume(ctx, expectedVolumePercent)
	if err != nil {
		return nil, err
	}

	inputMethod := ime.EnglishUS
	currentInputMethod, err := ime.ActiveInputMethod(ctx, tconn)
	if err != nil {
		if !errors.Is(err, ime.ErrInputNotDefined) {
			return nil, err
		}
		testing.ContextLog(ctx, "Failed to get active input method: ", err)
	} else {
		testing.ContextLog(ctx, "Current input method: ", currentInputMethod.Name)
	}
	if currentInputMethod == nil || !currentInputMethod.Equal(inputMethod) {
		testing.ContextLogf(ctx, "Install and activate %q input method", inputMethod.Name)
		if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
			return nil, err
		}
	}

	return func(ctx context.Context) error {
		if setVolumeErr := setVolumeNormal(ctx); setVolumeErr != nil {
			return errors.Wrap(setVolumeErr, "failed to reset volume setting")
		}
		return nil
	}, nil
}

// SetAudioVolume sets the audio volume to the expected percentage of the maximum volume and returns a function that restores the original volume.
func SetAudioVolume(ctx context.Context, expectedVolumePercent int) (action.Action, error) {
	vh, err := audio.NewVolumeHelper(ctx)
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
	name := fmt.Sprintf("reset volume to original volume: %d", originalVolume)
	return uiauto.NamedAction(name, func(ctx context.Context) error {
		return vh.SetVolume(ctx, originalVolume)
	}), nil
}
