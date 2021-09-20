// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

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
	defer func(ctx context.Context) {
		s.Log("Resetting brightness")
		if err := brightness.SetBrightnessMax(ctx); err != nil {
			s.Fatal("Failed to set brightness max during cleanup: ", err)
		}
	}(ctx)
	if err := brightness.SetBrightnessMax(ctx); err != nil {
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
	if err := brightness.KeyboardBrightnessTest(ctx, kb, topRow.BrightnessDown); err != nil {
		s.Fatal("Failed to decrease system brightness: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		decBrightness, err := brightness.GetSystemBrightness(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get system brightness"))
		}
		if decBrightness != 0.0 {
			return errors.Errorf("expected brightness 0.0 but got %q", decBrightness)
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
		s.Fatal("Failed to decrease brightness: ", err)
	}
	// Increasing brightness level with on-board keyboard key press.
	if err := brightness.KeyboardBrightnessTest(ctx, kb, topRow.BrightnessUp); err != nil {
		s.Fatal("Failed to increase system brightness: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		incBrightness, err := brightness.GetSystemBrightness(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get system brightness"))
		}
		if incBrightness != 100.0 {
			return errors.Errorf("expected brightness 100.0 but got %q", incBrightness)
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
		s.Fatal("Failed to increase brightness: ", err)
	}

}
