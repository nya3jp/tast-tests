// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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

	// For debugging purposes, log servo and dut connection type.
	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("Failed to find servo type: ", err)
	}
	s.Logf("Servo type: %s", servoType)

	dutConnType, err := h.Servo.GetDUTConnectionType(ctx)
	if err != nil {
		s.Fatal("Failed to find dut connection type: ", err)
	}
	s.Logf("DUT connection type: %s", dutConnType)

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

		// When charger disconnects/reconnects, there's a temporary drop in connection with the DUT.
		// Wait for DUT to reconnect before proceeding to the next step.
		waitConnectShortCtx, cancelWaitConnectShort := context.WithTimeout(ctx, 1*time.Minute)
		defer cancelWaitConnectShort()
		if err := h.WaitConnect(waitConnectShortCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		// Verify that DUT's charger was plugged/unplugged as expected.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			currentCharger, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return err
			} else if currentCharger != hasPluggedAC {
				return errors.Errorf("expected charger attached: %t, but got: %t", hasPluggedAC, currentCharger)
			}
			return nil
		}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second}); err != nil {
			s.Fatal("While determining charger state: ", err)
		}

		// For debugging purposes, log kernal output to check if hibernation is supported,
		// prior to suspending DUT.
		memSleep, err := h.DUT.Conn().CommandContext(ctx, "cat", "/sys/power/mem_sleep").Output()
		if err != nil {
			s.Log("Failed to read from '/sys/power/mem_sleep': ", err)
		}
		s.Logf("Output from '/sys/power/mem_sleep': %s", strings.TrimSpace(string(memSleep)))

		s.Log("Suspending DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend")
			if err := cmd.Start(); err != nil {
				return err
			}
			// Delay for some time to ensure the suspend command has fully propagated.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			// Check for DUT in S0ix or S3 powerstates.
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "S0ix", "S3"); err != nil {
				return errors.Wrap(err, "failed to get powerstates at S0ix or S3")
			}
			return nil
		}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
			s.Fatal("Failed to suspend DUT: ", err)
		}

		s.Log("Waiting for DUT to disconnect")
		if err := h.DisconnectDUT(ctx); err != nil {
			s.Fatal("Failed to disconnect DUT: ", err)
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
			if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
				s.Fatal("Failed to press ENTER key: ", err)
			}
		}

		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT after waking DUT from suspend: ", err)
		}

		// CCD might be locked after DUT has woken up.
		if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
			s.Fatal("While checking if servo has a CCD connection: ", err)
		} else if hasCCD {
			if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
				s.Fatal("Failed to get gsc_ccd_level: ", err)
			} else if val != servo.Open {
				s.Logf("CCD is not open, got %q. Attempting to unlock", val)
				if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
					s.Fatal("Failed to unlock CCD: ", err)
				}
			}
		}

		s.Log("Checking AC information")
		if ac, err := checkACInfo(ctx, h); err != nil {
			s.Fatal("While verifying ac information: ", err)
		} else {
			s.Logf("Line power %s", ac)
		}

		// For debugging purposes, also log information from the base file that
		// 'power_supply_info' derives battery state from.
		batteryBaseFile, err := checkBatteryFromBaseFile(ctx, h)
		if err != nil {
			s.Fatal("Could not determine battery status from base file: ", err)
		}

		s.Log("Checking battery information")
		reBatteryState := `state:(\s+\w+\s?\w+)`
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			battery, err := checkBatteryInfo(ctx, h, reBatteryState)
			if err != nil {
				return err
			}
			s.Logf("Battery state: %s", battery)
			switch hasPluggedAC {
			case true:
				if battery != "Fully charged" && battery != "Charging" {
					return errors.Errorf("found unexpected battery state when AC plugged: %s", battery)
				}
			case false:
				if battery != "Discharging" {
					return errors.Errorf("found unexpected battery state when AC unplugged: %s", battery)
				}
			}
			return nil
		}, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 2 * time.Minute}); err != nil {
			// In case that power_supply_info did not update as expected, validate battery state
			// based on the information found from the battery device path.
			s.Logf("While verifying battery information: %v. Attempting to check against base file", err)
			switch hasPluggedAC {
			case true:
				if batteryBaseFile != "Full" && batteryBaseFile != "Charging" {
					s.Fatalf("Found unexpected battery state from base file when AC plugged: %s", batteryBaseFile)
				}
			case false:
				if batteryBaseFile != "Discharging" {
					s.Fatalf("Found unexpected battery state from base file when AC unplugged: %s", batteryBaseFile)
				}
			}
		}
	}
}

// checkACInfo runs the host command 'power_supply_info', and returns information relevant to
// the AC. For debugging purposes, at the moment, only the source and online status are checked.
func checkACInfo(ctx context.Context, h *firmware.Helper) (string, error) {
	acLines := map[string]*regexp.Regexp{
		"source": regexp.MustCompile(`enum type:(\s+\w+)`),
		"online": regexp.MustCompile(`online:(\s+\w+)`),
	}
	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve AC info from DUT")
	}
	var measurements []string
	for k, v := range acLines {
		match := v.FindStringSubmatch(string(out))
		if len(match) < 2 {
			return "", errors.Errorf("Did not match regex %q in %q", v, string(out))
		}
		acInfo := strings.TrimSpace(match[1])
		measurements = append(measurements, fmt.Sprintf("%s: %s", k, acInfo))
	}
	return strings.Join(measurements, ", "), nil
}

// checkBatteryInfo runs the host command 'power_supply_info'and returns information relevant
// to the battery, based on the passed in pattern.
func checkBatteryInfo(ctx context.Context, h *firmware.Helper, pattern string) (string, error) {
	expMatch := regexp.MustCompile(pattern)
	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve power supply info from DUT")
	}
	matches := expMatch.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return "", errors.Errorf("failed to match regex %q in %q", expMatch, string(out))
	}
	batteryInfo := strings.TrimSpace(matches[1])
	return batteryInfo, nil
}

// checkBatteryFromBaseFile returns battery state reported by the base file,
// which the host command 'power_supply_info' depends on.
func checkBatteryFromBaseFile(ctx context.Context, h *firmware.Helper) (string, error) {
	// Find the battery device path from 'power_supply_info'.
	reBatteryDevicePath := `Device: (Battery\n.*path:\s*\S*)`
	batteryDevicePath, err := checkBatteryInfo(ctx, h, reBatteryDevicePath)
	if err != nil {
		return "", errors.Wrap(err, "unable to find battery device path")
	}

	// Information on battery status can usually be found under 'batteryDevicePath/status'.
	str := strings.Split(strings.TrimSpace(batteryDevicePath), "\n")
	baseFilePath := strings.TrimLeft(strings.ReplaceAll(str[len(str)-1], " ", ""), "path:")
	statusPath := fmt.Sprintf("%s/%s", baseFilePath, "status")

	testing.ContextLogf(ctx, "Checking battery information from %s", statusPath)
	batteryBaseFile, err := h.Reporter.CatFile(ctx, statusPath)
	if err != nil {
		return "", errors.Wrap(err, "could not determine battery status")
	}
	testing.ContextLogf(ctx, "Battery state from base file: %s", batteryBaseFile)
	return batteryBaseFile, nil
}
