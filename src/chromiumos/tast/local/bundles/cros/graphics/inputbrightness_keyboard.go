// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/graphics/brightness"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputbrightnessKeyboard,
		Desc:         "Verifies system Brightness increase and decrease through onboard keyboard",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
	})
}

func InputbrightnessKeyboard(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	initialBrightness, err := brightness.Percent(ctx)
	if err != nil {
		s.Fatal("Failed to get initial brightness: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Resetting brightness")
		if err := brightness.SetPercent(ctx, initialBrightness); err != nil {
			s.Fatal("Failed to restore DUTs initial brightness: ", err)
		}
	}(cleanupCtx)

	if err := brightness.SetPercent(ctx, 100); err != nil {
		s.Fatal("Failed to set brightness to maximmum: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard object: ", err)
	}
	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}
	// Decreasing brightness level with on-board keyboard key press.
	for {
		preBrightness, err := brightness.Percent(ctx)
		if err != nil {
			s.Fatal("Failed to get brightness: ", err)
		}
		decBrightness, err := waitForChangeInBrightness(ctx, func() error {
			return kb.Accel(ctx, topRow.BrightnessDown)
		})
		if err != nil {
			s.Fatal(`Failed to change brightness after pressing "BrightnessDown": `, err)
		}
		if decBrightness >= preBrightness {
			s.Fatal("Failed to decrease the brightness")
		}
		if decBrightness == 0.0 {
			break
		}
	}
	// Increasing brightness level with on-board keyboard key press.
	for {
		preBrightness, err := brightness.Percent(ctx)
		if err != nil {
			s.Fatal("Failed to get brightness: ", err)
		}
		incBrightness, err := waitForChangeInBrightness(ctx, func() error {
			return kb.Accel(ctx, topRow.BrightnessUp)
		})
		if err != nil {
			s.Fatal(`Failed to change brightness after pressing "BrightnessUp": `, err)
		}
		if incBrightness <= preBrightness {
			s.Fatal("Failed to increase the brightness")
		}
		if incBrightness == 100.0 {
			break
		}
	}

}

// waitForChangeInBrightness waits for change in brightness value while calling doBrightnessChange function.
// doBrightnessChange does brightness value change with keyboard BrightnessUp/BrightnessDown keypress.
func waitForChangeInBrightness(ctx context.Context, doBrightnessChange func() error) (float64, error) {
	var curBrightness float64
	preBrightness, err := brightness.Percent(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get system brightness")
	}
	if err := doBrightnessChange(); err != nil {
		return 0.0, errors.Wrap(err, "failed in calling doBrightnessChange function")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		curBrightness, err = brightness.Percent(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get brightness after doBrightnessChange function is called"))
		}
		if preBrightness == curBrightness {
			return errors.New("brightness not changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return 0.0, errors.Wrap(err, "failed to wait for brightness change")
	}
	return curBrightness, nil
}
