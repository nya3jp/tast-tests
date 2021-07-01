// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECBattery,
		Desc:         "Check battery temperature, voltage, and current readings",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Vars:         []string{"servo"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
	})
}

func ECBattery(ctx context.Context, s *testing.State) {
	const (
		BatteryStatus           = "/sys/class/power_supply/%s/status"
		BatteryVoltageReading   = "/sys/class/power_supply/%s/voltage_now"
		BatteryCurrentReading   = "/sys/class/power_supply/%s/current_now"
		VoltagemVErrorMargin    = 300
		CurrentmAErrorMargin    = 300
		BatteryTempUpperBound   = 70
		BatteryTempLowerBound   = 0
		BatteryNameLookupScript = `
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
	var (
		batteryName    string
		batteryVoltage string
		batteryCurrent string
	)

	h := s.PreValue().(*pre.Value).Helper

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()

	s.Log("Checking for battery info in sysfs")
	batteryNameOut, err := h.DUT.Conn().Command("bash", "-c", BatteryNameLookupScript).Output(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve battery info from sysfs: ", err)
	}

	batteryName = strings.TrimSuffix(string(batteryNameOut), "\n")
	if batteryName == "" {
		s.Fatal("Cannot find battery in sysfs or device does not have battery installed!")
	}

	s.Log("Battery name is ", batteryName)
	batteryVoltage = fmt.Sprintf(BatteryVoltageReading, batteryName)
	batteryCurrent = fmt.Sprintf(BatteryCurrentReading, batteryName)

	s.Log("Checking if voltage from sysfs matches servo")
	servoVoltageReading, err := h.Servo.GetInt(ctx, servo.BatteryVoltageMV)
	if err != nil {
		s.Fatal("Failed to read battery voltage from servo: ", err)
	}

	kernelVoltageReadingOut, err := h.DUT.Conn().Command("cat", batteryVoltage).Output(ctx)
	if err != nil {
		s.Fatal("Failed to read battery voltage from kernel: ", err)
	}

	kernelVoltageReadingStr := strings.TrimSuffix(string(kernelVoltageReadingOut), "\n")
	kernelVoltageReading, err := strconv.ParseInt(string(kernelVoltageReadingStr), 10, 64)
	if err != nil {
		s.Fatal(fmt.Sprintf("Failed to parse kernel voltage reading value ", kernelVoltageReadingStr, ": ", err))
	}

	kernelVoltageReading = kernelVoltageReading / 1000

	s.Log("Voltage reading from servo: ", servoVoltageReading, "mV")
	s.Log("Voltage reading from kernel: ", kernelVoltageReading, "mV")

	if math.Abs(servoVoltageReading-kernelVoltageReading) > VoltagemVErrorMargin {
		s.Fatal(fmt.Sprintf(
			"Voltage reading from servo (%dmV) and kernel (%dmV) mistmatch beyond %dmV error margin",
			servoVoltageReading,
			kernelVoltageReading,
			VoltagemVErrorMargin))
	}

	s.Log("Checking if current from sysfs matches servo")
	servoCurrentReading, err := h.Servo.GetInt(ctx, "ppvar_vbat_ma")
	if err != nil {
		s.Fatal("Failed to read battery current from servo: ", err)
	}

	kernelCurrentReadingOut, err := h.DUT.Conn().Command("cat", batteryCurrent).Output(ctx)
	if err != nil {
		s.Fatal("Failed to read battery current from kernel: ", err)
	}

	kernelCurrentReadingStr := strings.TrimSuffix(string(kernelCurrentReadingOut), "\n")
	kernelCurrentReading, err := strconv.ParseInt(string(kernelCurrentReadingStr), 10, 64)
	if err != nil {
		s.Fatal("Failed to parse kernel current reading value ", kernelCurrentReadingStr, " :", err)
	}
	kernelCurrentReading = kernelCurrentReading / 1000

	s.Log("Current reading from servo: ", servoCurrentReading, "mA")
	s.Log("Current reading from kernel: ", kernelCurrentReading, "mA")
	if math.Abs(servoCurrentReading-kernelCurrentReading) > CurrentmAErrorMargin {
		s.Fatal(fmt.Sprintf(
			"Current reading from servo (%dmA) and kernel (%dmA) mismatch beyond %dmA error margin",
			servoCurrentReading,
			kernelCurrentReading,
			CurrentmAErrorMargin))
	}

	s.Log("Checking if battery temperature is reasonable")
	batteryTemperatureOut, err := h.Servo.GetString(ctx, "battery_tempc")
	if err != nil {
		s.Fatal("Failed to read battery temperature from servo: ", err)
	}

	batteryTemperature, err := strconv.ParseFloat(batteryTemperatureOut, 64)
	if err != nil {
		s.Fatal(fmt.Sprintf("Failed to parse battery temperature from servo (%d) as float: ",
			batteryTemperatureOut,
			err))
	}
	s.Log("Battery temperature: ", batteryTemperature, " C")

	if batteryTemperature > BatteryTempUpperBound || batteryTemperature < BatteryTempLowerBound {
		s.Fatal(fmt.Sprintf(
			"Abnormal battery temperature %d (should be within %d-%d C)",
			batteryTemperature,
			BatteryTempLowerBound,
			BatteryTempUpperBound))
	}
}
