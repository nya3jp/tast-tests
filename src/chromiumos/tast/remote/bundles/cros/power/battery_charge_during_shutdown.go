// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/power"
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
		ServiceDeps:  []string{"tast.cros.power.BatteryService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      30 * time.Minute,
	})
}

// BatteryChargeDuringShutdown verifies battery charging is happening
// or not during DUT shutdown state.
func BatteryChargeDuringShutdown(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}

	origPdRole, err := h.Servo.GetPDRole(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve original USB PD role for Servo: ", err)
	}
	if origPdRole == servo.PDRoleNA {
		s.Fatal("Test requires Servo V4 or never to for operating DUT power delivery role through servo_pd_role")
	}

	cl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	client := power.NewBatteryServiceClient(cl.Conn)
	if _, err := client.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.Close(cleanupCtx, &empty.Empty{})

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
		}
		s.Log("Getting back to original USB PD role")
		if err := h.Servo.SetPDRole(ctx, origPdRole); err != nil {
			s.Fatal("Failed to get back to original USB PD role: ", err)
		}
	}(cleanupCtx)

	s.Log("Stopping power supply")
	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to stop power supply: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cs := chargingState(ctx, s, h)
		if cs["global.ac"] != "0" {
			return errors.New("DUT is not unplugged from AC charger")
		}
		if cs["global.state"] != "discharge" {
			return errors.New("DUT is not discharging (DUT is not on AC but does not report discharging)")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to unplug power supply: ", err)
	}

	cs := chargingState(ctx, s, h)
	ecBatteryPercentBeforeShutdown := chargingInt(cs["batt.state_of_charge"], "%")

	// Draining the battery charge percentage.
	if ecBatteryPercentBeforeShutdown >= 95 {
		s.Log("Draining battery")
		request := power.BatteryRequest{MaxPercentage: 90}
		if _, err := client.DrainBattery(ctx, &request); err != nil {
			s.Fatal("Failed to drain battery: ", err)
		}
		cs = chargingState(ctx, s, h)
		ecBatteryPercentBeforeShutdown = chargingInt(cs["batt.state_of_charge"], "%")
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
	s.Log("Starting power supply after shutdown")
	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to plug power supply via Servo-V4: ", err)
	}

	// Checking whether battery is able to charge in shutdown state.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cs = chargingState(ctx, s, h)
		if cs["global.ac"] != "1" {
			return errors.New("DUT is not plugged to AC charger")
		}
		if cs["global.state"] != "charge" {
			return errors.New("DUT is not charging (DUT is on AC but does not report charging)")
		}
		ecBatteryPercent := chargingInt(cs["batt.state_of_charge"], "%")
		if ecBatteryPercent <= ecBatteryPercentBeforeShutdown {
			return errors.Wrap(err, "failed to charge DUT battery")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Minute, Interval: 2 * time.Second}); err != nil {
		s.Fatal("Failed waiting to charge battery: ", err)
	}

	// Power on DUT after performing shutdown.
	if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
		s.Fatal("Failed to power on DUT: ", err)
	}

	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to unplug power supply via Servo-V4 during shutdown: ", err)
	}

	// Read battery info after shutdown with charger unplugged.
	batteryPercentAfterShutdown, err := batteryPercentage(ctx, dut)
	if err != nil {
		s.Fatal("Failed to read battery info after shutdown: ", err)
	}

	if batteryPercentAfterShutdown < batteryPercentBeforeShutdown {
		s.Fatal("Failed to charge DUT during shutdown")
	}
}

// batteryPercentage returns battery percentage info of DUT.
func batteryPercentage(ctx context.Context, dut *dut.DUT) (float64, error) {
	out, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get power supply info")
	}
	var matches []string
	batteryPercentRe := regexp.MustCompile(`^\s*percentage:\s+([0-9.]+)`)
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		match := batteryPercentRe.FindStringSubmatch(sc.Text())
		if match == nil {
			continue
		}
		matches = append(matches, match[1])
	}

	if len(matches) < 1 {
		return 0.0, errors.Wrap(err, "failed to find battery percent value")
	}
	batteryPercent := matches[0]
	curBatteryPercent, err := strconv.ParseFloat(batteryPercent, 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to convert from string to float")
	}
	return curBatteryPercent, nil
}

// chargingState returns map[string]string of parsed chgstate output from EC.
func chargingState(ctx context.Context, s *testing.State, h *firmware.Helper) map[string]string {
	chgstateOutput, err := h.Servo.RunECCommandGetOutput(ctx, "chgstate", []string{`.*\ndebug output = .+\n`})
	if err != nil {
		s.Fatal("Failed querying EC: ", err)
	}

	var (
		category string
		key      string
		value    string
	)

	cstateMap := make(map[string]string)

	// For reference, the current output of "chgstate" EC command is provided below
	// in shortened form.
	// Example output of "chgstate":
	//   state = charge
	//   ac = 1
	//   batt_is_charging = 1
	//   chg.*:
	//     voltage = 8648mV
	//     current = 0mA
	//     (...)
	//   batt.*:
	//     temperature = 24C
	//     state_of_charge = 100%
	//     voltage = 8543mV
	//     current = 0mA
	//     (...)
	//   requested_voltage = 0mV
	//   requested_current = 0mA
	//   chg_ctl_mode = 0
	//   (...)
	for _, line := range strings.Split(chgstateOutput[0][0], "\n") {
		if strings.Contains(line, "*") {
			category = strings.Split(line, ".")[0]
		}
		if strings.Contains(line, "=") {
			if !strings.HasPrefix(line, "\t") {
				category = "global"
			}

			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSpace(line)
			key = strings.Split(line, " = ")[0]
			value = strings.Split(line, " = ")[1]

			cstateMap[category+"."+key] = value
		}
	}
	return cstateMap
}

// chargingInt trims suffix, converts value to integer and returns charging value.
func chargingInt(raw, suffix string) (value int) {
	raw = strings.TrimSuffix(raw, suffix)
	value, _ = strconv.Atoi(raw)
	return value
}
