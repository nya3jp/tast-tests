// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECBattery,
		Desc:         "Check battery temperature, voltage, and current readings",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
	})
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func ECBattery(ctx context.Context, s *testing.State) {
	const (
		BatteryStatusFPTemplate             = "/sys/class/power_supply/%s/status"
		BatteryVoltageReadingFPTemplate     = "/sys/class/power_supply/%s/voltage_now"
		BatteryCurrentReadingFPTemplate     = "/sys/class/power_supply/%s/current_now"
		VoltageMVErrorMargin                = 300
		CurrentMAErrorMargin                = 300
		BatteryTemperatureCelsiusUpperBound = 70 // Temperature in Celsius
		BatteryTemperatureCelsiusLowerBound = 0  // Temperature in Celsius
		BatteryNameLookupScript             = `
			for path in $(grep -ilH --color=no Battery /sys/class/power_supply/*/type) ; do
				batteryName=$(basename $(dirname $path))
				if [   -e /sys/class/power_supply/$batteryName/status \
					-a -e /sys/class/power_supply/$batteryName/voltage_now \
					-a -e /sys/class/power_supply/$batteryName/current_now ] ; then
					echo $batteryName
					break
				fi
			done
		`
	)

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}

	s.Log("Checking for battery info in sysfs")
	batteryNameOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", BatteryNameLookupScript).Output()
	if err != nil {
		s.Fatal("Failed to retrieve battery info from sysfs: ", err)
	}

	batteryName := bytes.TrimSuffix(batteryNameOut, []byte("\n"))
	if batteryName == nil {
		s.Fatal("Cannot find battery in sysfs or device does not have battery installed")
	}

	s.Log("Battery name is ", string(batteryName))
	batteryVoltageFP := fmt.Sprintf(BatteryVoltageReadingFPTemplate, batteryName)
	batteryCurrentFP := fmt.Sprintf(BatteryCurrentReadingFPTemplate, batteryName)

	type comparisonTestCase struct {
		metric       string
		unit         string
		servoControl servo.IntControl
		sysfsPath    string
		errorMargin  int
	}

	for _, tc := range []comparisonTestCase{
		{"voltage", "mV", servo.BatteryVoltageMV, batteryVoltageFP, VoltageMVErrorMargin},
		{"current", "mA", servo.BatteryCurrentMA, batteryCurrentFP, CurrentMAErrorMargin},
	} {
		s.Logf("Checking if %s from sysfs matches servo", tc.metric)
		servoReading, err := h.Servo.GetInt(ctx, tc.servoControl)
		if err != nil {
			s.Fatalf("Failed to read battery %s from servo: %s", tc.metric, err)
		}
		servoReading = abs(servoReading)

		kernelReadingOut, err := h.DUT.Conn().CommandContext(ctx, "cat", tc.sysfsPath).Output()
		if err != nil {
			s.Fatalf("Failed to read battery %s from servo: %s", tc.metric, err)
		}
		kernelReadingOut = bytes.TrimSuffix(kernelReadingOut, []byte("\n"))
		kernelReading, err := strconv.Atoi(string(kernelReadingOut))
		if err != nil {
			s.Fatalf("Failed to parse kernel %s reading value %s: %s", tc.metric, kernelReadingOut, err)
		}

		// Kernel gives values in micro-units, convert to milli-units here.
		kernelReading = kernelReading / 1000
		kernelReading = abs(kernelReading)

		s.Logf("Battery %s reading from kernel: %d%s", tc.metric, kernelReading, tc.unit)
		s.Logf("Battery %s reading from servo: %d%s", tc.metric, servoReading, tc.unit)

		if abs(servoReading-kernelReading) > tc.errorMargin {
			s.Fatalf("Voltage reading from servo (%d%s) and kernel (%d%s) mismatch beyond %d%s error margin",
				servoReading, tc.unit, kernelReading, tc.unit, tc.errorMargin, tc.unit)
		}
	}

	s.Log("Checking if battery temperature is reasonable")
	batteryTemperature, err := h.Servo.GetFloat(ctx, servo.BatteryTemperatureCelsius)
	if err != nil {
		s.Fatal("Failed to read battery temperature from servo: ", err)
	}
	s.Log("Battery temperature: ", batteryTemperature, " C")

	if batteryTemperature > BatteryTemperatureCelsiusUpperBound ||
		batteryTemperature < BatteryTemperatureCelsiusLowerBound {
		s.Fatalf("Abnormal battery temperature %0.2f (should be within %d-%d C)",
			batteryTemperature,
			BatteryTemperatureCelsiusLowerBound,
			BatteryTemperatureCelsiusUpperBound)
	}
}
