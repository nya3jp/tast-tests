// Copyright 2021 The ChromiumOS Authors
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
		Timeout:      10 * time.Minute,
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

	// Increase timeout in getting response from ec uart.
	if err := h.Servo.SetString(ctx, "ec_uart_timeout", "10"); err != nil {
		s.Fatal("Failed to extend ec uart timeout: ", err)
	}
	defer func() {
		testing.ContextLog(ctx, "Restoring ec uart timeout to the default value of 3 seconds")
		if err := h.Servo.SetString(ctx, "ec_uart_timeout", "3"); err != nil {
			s.Fatal("Failed to restore default ec uart timeout: ", err)
		}
	}()

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
		waitConnectShortCtx, cancelWaitConnectShort := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelWaitConnectShort()
		if err := h.WaitConnect(waitConnectShortCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		// Stainless results showed that after setting dut power off, charger
		// was still attached to some duts. For debugging purposes, check whether
		// servo has reported false information about its control over pd role.
		hasControl, err := h.Servo.HasControl(ctx, string(servo.PDRole))
		if err != nil {
			s.Fatal("Failed to check for control: ", err)
		} else if hasControl && servoType == "type-a" {
			s.Log("Servo reported that it has control on pd role for Type-A")
		}

		// Verify that DUT's charger was plugged/unplugged as expected.
		var checkACInformation bool
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			checkACInformation = false
			currentCharger, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return err
			} else if currentCharger != hasPluggedAC {
				checkACInformation = true
				return errors.Errorf("expected charger attached: %t, but got: %t", hasPluggedAC, currentCharger)
			}
			return nil
		}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 1 * time.Second}); err != nil {
			if checkACInformation {
				// For debugging purposes, log charger information if the charger
				// state is unexpected and before failing the test.
				s.Log("Running host command to check charger information")
				if ac, err := checkACInfo(ctx, h); err != nil {
					s.Fatal("Unable to read ac information: ", err)
				} else {
					s.Logf("Line power %s", ac)
				}
				s.Log("Running ec command to check for charger state")
				if err := checkECChgState(ctx, h); err != nil {
					s.Fatal("Failed to query ec chgstate: ", err)
				}
			}
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
			// Check for DUT in S0ix, S3, S5, or G3 power state.
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "S0ix", "S3", "S5", "G3"); err != nil {
				// When tested on one of the devices leased from the lab, for example trogdor[lazor],
				// connection to the DUT dropped, but the DUT's power state remained at S0. Check if
				// this would also happen on other DUTs, and if so document this behavior in the returned
				// error message.
				if !h.DUT.Connected(ctx) {
					return errors.New("did not get power state at S0ix, S3, S5 or G3, but got dut disconnected")
				}
				return errors.Wrap(err, "failed to get power state at S0ix, S3, S5, or G3")
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

		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			checkPowerState := func() string {
				s.Log("Checking for the DUT's power state")
				state, err := h.Servo.GetECSystemPowerState(ctx)
				if err != nil {
					s.Log("Error getting power state: ", err)
					return "unknown"
				}
				return state
			}
			value := checkPowerState()
			s.Fatalf("Failed to reconnect to DUT after waking DUT from suspend: %v, got DUT at power state: %s", err, value)
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

		s.Log("Checking charger information")
		if ac, err := checkACInfo(ctx, h); err != nil {
			s.Fatal("While verifying ac information: ", err)
		} else {
			s.Logf("Line power %s", ac)
		}

		s.Log("Checking ec charger state")
		if err := checkECChgState(ctx, h); err != nil {
			s.Fatal("Failed to query ec chgstate: ", err)
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
// the line power section.
func checkACInfo(ctx context.Context, h *firmware.Helper) (string, error) {
	// Regular expression.
	re := regexp.MustCompile(`Device:\s+Line Power(\s+)(\n|.)*?supports dual-role:\s+\w+`)
	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve AC info from DUT")
	}
	match := re.FindStringSubmatch(string(out))
	if match[0] == "" {
		return "", errors.New("no regexp match found")
	}
	chargerInfoMap := make(map[string]string)
	for _, val := range strings.Split(match[0], "\n") {
		val := strings.Split(val, ":")
		chargerInfoKey := strings.TrimSpace(val[0])
		chargerInfoVal := strings.TrimSpace(val[1])
		chargerInfoMap[strings.ReplaceAll(chargerInfoKey, " ", "_")] = chargerInfoVal
	}
	// Expand wantData if more information is needed.
	var infoStr string
	wantData := []string{"enum_type", "type", "online", "active_source", "available_sources"}
	for _, name := range wantData {
		if attr, ok := chargerInfoMap[name]; ok && attr != "" {
			infoStr += fmt.Sprintf("%s:%s", name, chargerInfoMap[name]) + "; "
		}
	}
	return infoStr, nil
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

// checkECChgState sends command 'chgstate' to the ec and prints returned output,
// when the listed regular expressions are matched.
func checkECChgState(ctx context.Context, h *firmware.Helper) error {
	// Regular expressions to check for battery and charger state.
	var (
		reAC                       = `ac = (\S*)`
		reState                    = `state = (\S*)`
		reBatteryIsCharging        = `batt_is_charging = (\S*)`
		reBatterySeemsDead         = `battery\S*dead = (\S*)`
		reBatterySeemsDisconnected = `battery\S*disconnected = (\S*)`
		reBatteryRemoved           = `battery\S*removed = (\S*)`
	)
	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		return errors.Wrap(err, "failed to send 'chan 0' to EC")
	}
	defer func() error {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			return errors.Wrap(err, "failed to send 'chan 0xffffffff' to EC")
		}
		return nil
	}()
	chgstateOutput, err := h.Servo.RunECCommandGetOutput(ctx, "chgstate", []string{`.*\ndebug output = .+\n`})
	if err != nil {
		return err
	}
	regs := []*regexp.Regexp{regexp.MustCompile(reAC), regexp.MustCompile(reState), regexp.MustCompile(reBatteryIsCharging),
		regexp.MustCompile(reBatterySeemsDead), regexp.MustCompile(reBatterySeemsDisconnected), regexp.MustCompile(reBatteryRemoved)}
	var chgStateInfo []string
	for _, v := range regs {
		if match := v.FindStringSubmatch(chgstateOutput[0][0]); match != nil {
			chgStateInfo = append(chgStateInfo, match[0])
		}
	}
	testing.ContextLog(ctx, chgStateInfo)
	return nil
}
