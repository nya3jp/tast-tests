// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MenuPowerOff,
		Desc:         "Test power off from UI menu in developer and recovery screen (the test is skipped for DUTs that do not have such menu)",
		Contacts:     []string{"tj@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.DevMode,
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func MenuPowerOff(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	if switcher := h.Config.ModeSwitcherType; switcher != "menu_switcher" && switcher != "tablet_detachable_switcher" {
		s.Log("Test skipped for menuless UI, DUT's mode-switcher type is: ", switcher)
		return
	}
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		s.Fatal("Failed to restart to recovery mode: ", err)
	}
	s.Logf("Sleeping %s (FirmwareScreen)", h.Config.FirmwareScreen)
	if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	if err := powerOffFromUIMenu(ctx, h, "rec"); err != nil {
		s.Fatal("Failed to power off from recovery screen: ", err)
	}
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
		s.Fatal("Failed to restart: ", err)
	}
	s.Logf("Sleeping %s (FirmwareScreen)", h.Config.FirmwareScreen)
	if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	s.Log("Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		s.Fatal("Failed to get S0 powerstate: ", err)
	}
	if err := powerOffFromUIMenu(ctx, h, "dev"); err != nil {
		s.Fatal("Failed to power off from developer screen: ", err)
	}
}

func powerOffFromUIMenu(ctx context.Context, h *firmware.Helper, screen string) error {
	testing.ContextLog(ctx, "Choosing power off option from the UI menu")
	switch h.Config.ModeSwitcherType {
	case "menu_switcher":
		// Power off is the last option in the menu for both developer and recovery screen
		if err := nTimeKeyPress(ctx, h, "<down>", 7); err != nil {
			return errors.Wrap(err, "failed to type the key")
		}
	case "tablet_detachable_switcher":
		switch screen {
		case "rec":
			// Enter the menu by pressing <down> and choose power off option which is directly below the default option
			if err := nTimeKeyPress(ctx, h, "<down>", 2); err != nil {
				return errors.Wrap(err, "failed to type the key")
			}
		case "dev":
			// Power off is the default option
		}
	}
	if err := nTimeKeyPress(ctx, h, "<enter>", 1); err != nil {
		return errors.Wrap(err, "failed to type the key")
	}
	testing.ContextLog(ctx, "Waiting for power state to become G3 or S5")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}
	return nil
}

func nTimeKeyPress(ctx context.Context, h *firmware.Helper, key string, n int) error {
	for i := 0; i < n; i++ {
		if err := h.Servo.ECPressKey(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
