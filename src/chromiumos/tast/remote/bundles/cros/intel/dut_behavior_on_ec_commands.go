// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"regexp"
	"strings"
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
		Func:         DUTBehaviorOnECCommands,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies DUT behavior on executing various EC commands",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      12 * time.Minute,
	})
}

func DUTBehaviorOnECCommands(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	dutSupportsEC := false
	if err := dut.Conn().CommandContext(ctx, "ectool", "version").Run(); err == nil {
		dutSupportsEC = true
	}

	defer func(ctx context.Context) {
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to power on DUT at cleanup: ", err)
		}
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to connect charger: ", err)
		}
	}(cleanupCtx)

	s.Log("Unplugging power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to disconnect charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return errors.Wrap(err, "failed to get charger unplugged status")
		} else if attached {
			return errors.New("charger is not unplugged - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Failed to check if charger is disconnected via Servo V4: ", err)
	}

	powerInfoOut, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		s.Fatal("Failed to get battery info with power_supply_info command: ", err)
	}
	powerInfoRe := regexp.MustCompile(`state.*Discharging`)
	if !powerInfoRe.Match(powerInfoOut) {
		s.Fatal("Failed to check for battery discharge state")
	}

	// Performing DUT reboot.
	s.Log("Reboot DUT with 'reboot' command")
	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	// Performing DUT Shutdown.
	s.Log("Shut-down DUT with 'shutdown' command")
	ecPowerS5State := "S5"
	if err := powercontrol.ShutdownAndWaitForPowerState(ctx, h.ServoProxy, dut, ecPowerS5State); err != nil {
		s.Fatal("Failed to shutdown and wait for S5 state: ", err)
	}

	if dutSupportsEC {
		if err := performECPowerbtn(ctx, h, dut); err != nil {
			s.Fatal("Failed to power on DUT with EC 'powerbtn' command: ", err)
		}
	} else {
		if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
			s.Fatal("Failed to power on DUT with servo power normal press: ", err)
		}
	}

	s.Log("Perform power button long-press to shutdown DUT")
	if err := performServoPowerLongPress(ctx, h, dut); err != nil {
		s.Fatal("Failed to power long press via servo and power-on dut: ", err)
	}

	s.Log("Perform DUT cold-reset")
	if err := performColdReset(ctx, h, dut); err != nil {
		s.Fatal("Failed to perform cold-reset: ", err)
	}

	if !dutSupportsEC {
		return
	}

	ectoolOut, err := dut.Conn().CommandContext(ctx, "ectool", "chargestate", "show").Output()
	if err != nil {
		s.Fatal("Failed to get battery info with ectool command: ", err)
	}
	chargestateACOffString := "ac = 0"
	if !strings.Contains(string(ectoolOut), chargestateACOffString) {
		s.Fatal("Failed to check for battery charge state OFF")
	}

	s.Log("Rebooting DUT with EC console command 're'")
	if err := performRebootWithECCommand(ctx, h, dut); err != nil {
		s.Fatal("Failed to reboot DUT with 're' EC command: ", err)
	}

	s.Log("Rebooting with 'reboot ap-off' command")
	if err := performRebootApOff(ctx, h, dut); err != nil {
		s.Fatal("Failed to perform reboot ap-off: ", err)
	}

	s.Log("Shutdown with 'apshutdown' command")
	if err := performApShutdown(ctx, h, dut); err != nil {
		s.Fatal("Failed to perform apshutdown: ", err)
	}

	// Run EC command to put DUT in hibernate.
	s.Log("Keep DUT in hibernate state with 'hibernate 5' command")
	if err := performHiberate(ctx, h, dut); err != nil {
		s.Fatal("Failed to perform hibernation: ", err)
	}

	s.Log("Plugging power supply")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to connect charger: ", err)
	}
	if err := waitForChargerPlug(ctx, h); err != nil {
		s.Fatal("Failed to check if charger is connected via Servo V4: ", err)
	}

	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		s.Fatal("Failed wait for unreachable after hibernate: ", err)
	}

	s.Log("Warm boot DUT with 'apreset' command")
	if err := performApReset(ctx, h, dut); err != nil {
		s.Fatal("Failed to perform apreset: ", err)
	}
	cbmemSleepState := 0
	if err := powercontrol.ValidatePrevSleepState(ctx, dut, cbmemSleepState); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}
}

// performServoPowerLongPress performs long press power button via servo, waits for G3 state
// and power-on DUT with power normal press via servo.
func performServoPowerLongPress(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	powerOffCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := h.Servo.KeypressWithDuration(powerOffCtx, servo.PowerKey, servo.DurLongPress); err != nil {
		return errors.Wrap(err, "failed to power long press")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait DUT unreachable")
	}
	ecPowerG3State := "G3"
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, ecPowerG3State); err != nil {
		return errors.Wrap(err, "failed to verify EC power state after power long-press")
	}
	if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
		return errors.Wrap(err, "failed to power on DUT with servo power normal press after power long-press")
	}
	return nil
}

// performRebootApOff performs reboot DUT with reboot ap-off EC command.
func performRebootApOff(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if err := h.Servo.RunECCommand(ctx, "reboot ap-off"); err != nil {
		return errors.Wrap(err, "failed to execute 'reboot ap-off' command on EC console")
	}
	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT after reboot")
	}
	return nil
}

// performApShutdown performs shutdown with 'apshutdown' EC command, waits for G3 state
// and power-on DUT with power normal press via servo.
func performApShutdown(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if err := h.Servo.RunECCommand(ctx, "apshutdown"); err != nil {
		return errors.Wrap(err, "failed to execute 'apshutdown' command on EC console")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed wait for unreachable")
	}
	ecPowerG3State := "G3"
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, ecPowerG3State); err != nil {
		return errors.Wrap(err, "failed to verify EC power state after 'apshutdown'")
	}
	if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
		return errors.Wrap(err, "failed to power on DUT with servo after 'apshutdown'")
	}
	return nil
}

// performHiberate performs hibernate with 'hibernate 5' EC command and checks for
// EC console non-responsiveness after hibernating.
func performHiberate(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if err := h.Servo.RunECCommand(ctx, "hibernate 5"); err != nil {
		return errors.Wrap(err, "failed to run EC command")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed wait for unreachable")
	}
	testing.ContextLog(ctx, "Verify EC is non-responsive")
	if err := h.Servo.CheckUnresponsiveEC(ctx); err != nil {
		return errors.Wrap(err, "failed to check EC console to be non-resposive")
	}
	return nil
}

// performRebootWithECCommand perform DUT rebooting with 're' EC command and
// wait for DUT to connect back.
func performRebootWithECCommand(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if _, err := h.Servo.RunECCommandGetOutput(ctx, "re", []string{`Rebooting!`}); err != nil {
		return errors.Wrap(err, "failed to execute 're' command on EC console")
	}
	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT after reboot")
	}
	return nil
}

// performApReset performs warm boot with 'apreset' EC command and wait
// for DUT to connect back.
func performApReset(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if err := h.Servo.RunECCommand(ctx, "apreset"); err != nil {
		return errors.Wrap(err, "failed to execute 'apreset' command on EC console")
	}
	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT after apreset")
	}
	return nil
}

// performColdReset performs cold-reset with dut-control command and wait
// for DUT to connect back.
func performColdReset(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	if err := h.Servo.SetOnOff(ctx, servo.ColdReset, servo.On); err != nil {
		return errors.Wrap(err, "failed to set cold_reset to on")
	}
	if err := h.Servo.SetOnOff(ctx, servo.ColdReset, servo.Off); err != nil {
		return errors.Wrap(err, "failed to set cold_reset to off")
	}
	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT after reboot")
	}
	return nil
}

// performECPowerbtn performs powering ON DUT with 'powerbtn' EC command.
func performECPowerbtn(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := h.Servo.RunECCommandGetOutput(ctx, "powerbtn", []string{`Simulating 200 ms power button press\.`}); err != nil {
			return errors.Wrap(err, "failed to execute 'powerbtn' command on EC console")
		}
		// Context for waiting for DUT connect.
		wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := dut.WaitConnect(wtCtx); err != nil {
			return errors.Wrap(err, "failed to wait for connect")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}

// waitForChargerPlug will poll for getting charger plugged status.
func waitForChargerPlug(ctx context.Context, h *firmware.Helper) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return errors.Wrap(err, "failed to get charger plug status after hiberate")
		} else if !attached {
			return errors.New("charger is not plugged - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond})
}
