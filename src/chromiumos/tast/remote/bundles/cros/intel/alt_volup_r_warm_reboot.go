// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AltVolupRWarmReboot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test if 'Alt + Vol Up + R' warm reboots the DUT successfully",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.NormalMode,
		Timeout:      5 * time.Minute,
	})
}

func AltVolupRWarmReboot(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, h.DUT, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	// Press three keys together: Alt + Vol Up + R
	if err := func(ctx context.Context) error {
		for _, targetKey := range []string{"<alt_l>", "<f10>", "r"} {
			row, col, err := h.Servo.GetKeyRowCol(targetKey)
			if err != nil {
				return errors.Wrapf(err, "failed to get key %s column and row", targetKey)
			}
			targetKeyName := targetKey
			targetKeyHold := fmt.Sprintf("kbpress %d %d 1", col, row)
			targetKeyRelease := fmt.Sprintf("kbpress %d %d 0", col, row)
			s.Logf("Pressing and holding key %s", targetKey)
			if err := h.Servo.RunECCommand(ctx, targetKeyHold); err != nil {
				return errors.Wrapf(err, "failed to press and hold key %s", targetKey)
			}

			defer func(releaseKey, name string) error {
				s.Logf("Releasing key %s", name)
				if err := h.Servo.RunECCommand(ctx, releaseKey); err != nil {
					return errors.Wrapf(err, "failed to release key %s", releaseKey)
				}
				return nil
			}(targetKeyRelease, targetKeyName)
		}
		return nil
	}(ctx); err != nil {
		s.Fatal("Failed to press keys: ", err)
	}

	s.Log(ctx, "Waiting for DUT to shutdown")
	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := h.DUT.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to shutdown DUT: ", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := h.DUT.WaitConnect(waitCtx); err != nil {
		s.Fatal("Failed to wait connect DUT: ", err)
	}

	// Performing prev_sleep_state check.
	if err := powercontrol.ValidatePrevSleepState(ctx, h.DUT, 0); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}
}
