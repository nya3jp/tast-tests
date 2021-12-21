// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BatteryCharging,
		Desc:         "Verify battery information when charger state is changed during suspend",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
	})
}

func BatteryCharging(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	// checkPowerInfo checks the power supply information after waking up DUT from suspend.
	checkPowerInfo := func(ctx context.Context) (string, error) {
		s.Log("Checking for DUT's powerstate at S0")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
			return "", errors.Wrap(err, "failed to get powerstate at S0 after waking DUT")
		}

		regex := `state:(\s+\w+\s?\w+)`
		expMatch := regexp.MustCompile(regex)

		out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
		if err != nil {
			return "", errors.Wrap(err, "failed to retrieve power supply info from DUT")
		}

		matches := expMatch.FindStringSubmatch(string(out))
		if len(matches) < 2 {
			return "", errors.Errorf("failed to match regex %q in %q", expMatch, string(out))
		}

		batteryStatus := strings.TrimSpace(matches[1])
		return batteryStatus, nil
	}

	for _, tc := range []struct {
		plugAC     bool
		wakeSource string
	}{
		{false, "plugging AC"},
		{true, "unplugging AC"},
	} {
		s.Logf("Plug in AC: %t", tc.plugAC)
		if err := h.SetDUTPower(ctx, tc.plugAC); err != nil {
			s.Fatal("Failed to set DUT power: ", err)
		}
		hasPluggedAC := tc.plugAC

		s.Log("Suspending DUT")
		cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend")
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to suspend DUT: ", err)
		}

		s.Log("Checking for DUT in S0ix or S3 powerstates")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
			s.Fatal("Failed to get powerstates at S0ix or S3: ", err)
		}

		if h.Config.ModeSwitcherType == firmware.MenuSwitcher && h.Config.Platform != "zork" {
			s.Logf("Waking DUT from suspend by %s", tc.wakeSource)
			switch tc.wakeSource {
			case "plugging AC":
				if err := h.SetDUTPower(ctx, true); err != nil {
					s.Fatal("Failed to connect charger: ", err)
				}
				hasPluggedAC = true
			case "unplugging AC":
				if err := h.SetDUTPower(ctx, false); err != nil {
					s.Fatal("Failed to remove charger: ", err)
				}
				hasPluggedAC = false
			}
		} else if h.Config.ModeSwitcherType == firmware.TabletDetachableSwitcher {
			s.Log("Waking DUT from suspend by a tab on power button")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				s.Fatal("Failed to press power button: ", err)
			}
		} else {
			// Old devices would not wake from plugging/unplugging AC.
			// Instead, we replace by pressing a keyboard key.
			s.Log("Waking DUT from suspend by pressing ENTER key")
			if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
				s.Fatal("Failed to press ENTER key: ", err)
			}
		}

		battery, err := checkPowerInfo(ctx)
		if err != nil {
			s.Fatal("While verifying power supply information: ", err)
		}

		s.Log("Checking power supply information")
		switch hasPluggedAC {
		case true:
			if battery != "Fully charged" && battery != "Charging" {
				s.Fatalf("Found unexpected battery state when AC plugged: %s", battery)
			}
		case false:
			if battery != "Discharging" {
				s.Fatalf("Found unexpected battery state when AC unplugged: %s", battery)
			}
		}
		s.Logf("Battery state: %q", battery)
	}
}
