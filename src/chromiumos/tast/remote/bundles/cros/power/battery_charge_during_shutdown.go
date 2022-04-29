// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
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
		Func:         BatteryChargeDuringShutdown,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies battery is charging during DUT shutdown",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      7 * time.Minute,
	})
}

// BatteryChargeDuringShutdown verifies battery charging is happening
// or not during DUT shutdown state.
func BatteryChargeDuringShutdown(ctx context.Context, s *testing.State) {
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

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powerNormalPress(ctx, h, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		if err := plugUnplugCharger(ctx, h, true); err != nil {
			s.Fatal("Failed to plug power supply via Servo-V4 at cleanup: ", err)
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Stopping power supply before Chrome login")
	if err := plugUnplugCharger(ctx, h, false); err != nil {
		s.Fatal("Failed to unplug power supply via Servo-V4: ", err)
	}

	// Perform a Chrome login.
	testing.ContextLog(ctx, "Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	// Read battery info before shutdown with charger unplugged.
	batteryPercentBeforeShutdown, err := batteryPercentage(ctx, dut)
	if err != nil {
		s.Fatal("Failed to read battery info before shutdown: ", err)
	}

	// Performing DUT Shutdown.
	if err := dut.Conn().CommandContext(ctx, "shutdown", "-h", "now").Run(); err != nil {
		s.Fatal("Failed to execute shutdown command: ", err)
	}
	sdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to wait for unreachable: ", err)
	}

	// During DUT shutdown state plug power supply via Servo-V4
	testing.ContextLog(ctx, "Starting power supply after shutdown")
	if err := plugUnplugCharger(ctx, h, true); err != nil {
		s.Fatal("Failed to plug power supply via Servo-V4: ", err)
	}

	// Keeping DUT sleep for 5 minutes in shutdown state and
	// check battery reading changed after powering on DUT.
	testing.ContextLog(ctx, "DUT sleeping for 5 minutes")
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := plugUnplugCharger(ctx, h, false); err != nil {
		s.Fatal("Failed to unplug power supply via Servo-V4 during shutdown: ", err)
	}

	// Power on DUT after unplugging power supply.
	if err := powerNormalPress(ctx, h, dut); err != nil {
		s.Fatal("Failed to power on DUT: ", err)
	}

	// Read battery info after shutdown with charger unplugged.
	batteryPercentAfterShutdown, err := batteryPercentage(ctx, dut)
	if err != nil {
		s.Fatal("Failed to read battery info after shutdown: ", err)
	}

	if batteryPercentAfterShutdown <= batteryPercentBeforeShutdown {
		s.Fatal("Failed to charge DUT during shutdown: ", err)
	}
}

// plugUnplugCharger performs plugging/unplugging of charger via servo.
func plugUnplugCharger(ctx context.Context, h *firmware.Helper, isPowerPlugged bool) error {
	chargerStatus := ""
	if isPowerPlugged {
		chargerStatus = "not attached"
	} else {
		chargerStatus = "attached"
	}
	if err := h.SetDUTPower(ctx, isPowerPlugged); err != nil {
		return errors.Wrap(err, "failed to remove charger")
	}
	getChargerPollOptions := testing.PollOptions{Timeout: 10 * time.Second}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if isPowerPlugged != attached {
			return errors.Errorf("charger is still %q - use Servo V4 Type-C or supply RPM vars", chargerStatus)
		}
		return nil
	}, &getChargerPollOptions)
}

// powerNormalPress performs power button normal press to power on DUT via servo.
func powerNormalPress(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}

// batteryPercentage returns battery percentage info of DUT.
func batteryPercentage(ctx context.Context, dut *dut.DUT) (float64, error) {
	out, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get power supply info")
	}
	batteryPercentRe := regexp.MustCompile(`percentage:\s+([0-9.]+)`)
	matches := batteryPercentRe.FindStringSubmatch(string(out))
	if len(matches) < 1 {
		return 0.0, errors.Wrap(err, "failed to find battery percent value")
	}
	batteryPercent := matches[1]
	curBatteryPercent, err := strconv.ParseFloat(batteryPercent, 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to convert from string to float")
	}
	return curBatteryPercent, nil
}
