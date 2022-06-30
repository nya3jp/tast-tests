// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	fwRemote "chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type menuType struct {
	mode         string
	switcherType fwRemote.ModeSwitcherType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MenuPowerOff,
		Desc:         "Test power off from UI menu in developer and recovery screen (the test is skipped for DUTs that do not have such menu)",
		Contacts:     []string{"tj@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.DevMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func MenuPowerOff(ctx context.Context, s *testing.State) {
	supportedModeSwitcherTypes := []fwRemote.ModeSwitcherType{"menu_switcher", "tablet_detachable_switcher"}

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	menulessUI := true
	for _, v := range supportedModeSwitcherTypes {
		if h.Config.ModeSwitcherType == v {
			menulessUI = false
		}
	}
	if !menulessUI {
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
			s.Fatal("Failed to restart to recovery mode: ", err)
		}
		powerOffFromUIMenu(ctx, h, s, "rec")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Fatal("Failed to restart: ", err)
		}
		s.Logf("Sleeping %s (FirmwareScreen)", h.Config.FirmwareScreen)
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
		s.Log("Waiting for S0 powerstate")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
			s.Fatal("Failed to get S0 powerstate")
		}
		powerOffFromUIMenu(ctx, h, s, "dev")
	} else {
		s.Log("Test skipped for menuless UI, DUT's mode-switcher type is: ", h.Config.ModeSwitcherType)
	}
}

func powerOffFromUIMenu(ctx context.Context, h *firmware.Helper, s *testing.State, screen string) {
	s.Log("Choosing power off option from the UI menu")
	switch {
	case h.Config.ModeSwitcherType == "menu_switcher":
		// Power off is the last option in the menu for both developer and recovery screen
		nTimeKeyPress(ctx, h, s, "<down>", 7)
	case h.Config.ModeSwitcherType == "tablet_detachable_switcher" && screen == "rec":
		// Enter the menu by pressing <down> and choose power off option which is directly below the default option
		nTimeKeyPress(ctx, h, s, "<down>", 2)
	case h.Config.ModeSwitcherType == "tablet_detachable_switcher" && screen == "dev":
		// Power off is the default option
	}
	nTimeKeyPress(ctx, h, s, "<enter>", 1)
	s.Log("Waiting for power state to become G3 or S5")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		s.Fatal("Failed to get G3 or S5 powerstate: ", err)
	}
}

func nTimeKeyPress(ctx context.Context, h *firmware.Helper, s *testing.State, key string, n int) {
	for i := 0; i < n; i++ {
		if err := h.Servo.ECPressKey(ctx, key); err != nil {
			s.Fatal("Failed to type key: ", err)
		}
	}
}
