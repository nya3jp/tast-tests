// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootWithOnlyBattery,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies DUT boots with battery after unplugging AC power supply",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
	})
}

func BootWithOnlyBattery(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if attached {
			return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to check if charger is disconnected via Servo V4: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to attach charger: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return err
			} else if !attached {
				return errors.New("charger is not attached at cleanup - use Servo V4 Type-C or supply RPM vars")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to check if charger is connected via Servo V4: ", err)
		}
	}(cleanupCtx)

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	// If DUT has EC support check battery status with ectool commands.
	if hasECAccess(ctx, dut) {
		if err := verifyEctoolBattery(ctx, dut); err != nil {
			s.Fatal("Failed to verify ectool battery: ", err)
		}
		if err := verifyEctoolChargeState(ctx, dut); err != nil {
			s.Fatal("Failed to verify ectool chargestate show: ", err)
		}
	}

	// Check battery info with power_supply_info command.
	if err := verifyPowerSupplyInfo(ctx, dut); err != nil {
		s.Fatal("Failed to verify power supply info: ", err)
	}

	// Even after unplugging AC power supply, DUT has to be in S0 state.
	if err := verifyECPowerInfo(ctx, h); err != nil {
		s.Fatal("Failed to verify EC power state info via servo: ", err)
	}

	// Check battery info via servo.
	if err := verifyECBattery(ctx, h); err != nil {
		s.Fatal("Failed to verify EC battery info via servo: ", err)
	}
}

// verifyEctoolBattery checks ectool battery flag is discharging or not.
func verifyEctoolBattery(ctx context.Context, dut *dut.DUT) error {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "battery").Output()
	if err != nil {
		return errors.Wrap(err, "failed to get ectool battery info")
	}
	dischargeFlagRe := regexp.MustCompile(`Flags.*BATT_PRESENT.*DISCHARGING`)
	if !dischargeFlagRe.MatchString(string(out)) {
		return errors.New("unexpected battery flag: got charging, want discharging")
	}
	return nil
}

// verifyPowerSupplyInfo checks battery power supply status is discharging or not.
func verifyPowerSupplyInfo(ctx context.Context, dut *dut.DUT) error {
	out, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return errors.Wrap(err, "failed to get power supply info")
	}
	dischargeStateRe := regexp.MustCompile(`state.*Discharging`)
	if !dischargeStateRe.MatchString(string(out)) {
		return errors.New("unexpected power_supply_info state: got charging, want discharging")
	}
	return nil
}

// verifyEctoolChargeState checks ectool chargestate AC status is zero or not.
func verifyEctoolChargeState(ctx context.Context, dut *dut.DUT) error {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "chargestate", "show").Output()
	if err != nil {
		return errors.Wrap(err, "failed to get charge state info")
	}
	batteryACPowerRe := regexp.MustCompile("ac.*0")
	if !batteryACPowerRe.MatchString(string(out)) {
		return errors.New("unexpected AC flag in chargestate info: got 1, want 0")
	}
	return nil
}

// verifyECPowerInfo checks whether DUT is in S0 EC power state or not via servo.
func verifyECPowerInfo(ctx context.Context, h *firmware.Helper) error {
	got, err := h.Servo.GetECSystemPowerState(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get power state via servo")
	}
	if want := "S0"; got != want {
		return errors.Errorf("unexpected EC power state: got %s, want %s", got, want)
	}
	return nil
}

// verifyECBattery checks whether battery status is dicharge or not via servo.
func verifyECBattery(ctx context.Context, h *firmware.Helper) error {
	out, err := h.Servo.RunECCommandGetOutput(ctx, "battery", []string{`Status:.*DCHG.*`})
	if err != nil {
		return errors.Wrap(err, "failed to run command in EC console")
	}
	want := "DCHG"
	if len(out) == 0 {
		return errors.Wrap(err, "failed to get EC command output")
	}
	got := out[0][0]
	if !strings.Contains(got, want) {
		return errors.Errorf("unexpected EC battery info: got %s, want %s", got, want)
	}
	return nil
}

// hasECAccess return true if DUT has EC access to execute ectool commands.
func hasECAccess(ctx context.Context, dut *dut.DUT) bool {
	if err := dut.Conn().CommandContext(ctx, "ectool", "version").Run(); err != nil {
		return false
	}
	return true
}
