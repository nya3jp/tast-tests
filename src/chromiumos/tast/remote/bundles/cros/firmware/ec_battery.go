// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECBattery,
		Desc:         "Check battery temperature, voltage, and current readings",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Vars:         []string{"servo"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
	})
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsExist(err)
}

func ECBattery(ctx context.Context, s *testing.State) {
	const (
		BATTERY_STATUS           = "/sys/class/power_supply/%s/status"
		BATTERY_VOLTAGE_READING  = "/sys/class/power_supply/%s/voltage_now"
		BATTERY_CURRENT_READING  = "/sys/class/power_supply/%s/current_now"
		VOLTAGE_MV_ERROR_MARGIN  = 300
		CURRENT_MA_ERROR_MARGIN  = 300
		BATTERY_TEMP_UPPER_BOUND = 70
		BATTERY_TEMP_LOWER_BOUND = 0
	)
	var (
		batteryName    string
		batteryVoltage string
		batteryCurrent string
	)
	batteryRegexp := regexp.MustCompile("/sys/class/power_supply/([^/]+)/")

	d := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	if err = pxy.Servo().RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}

	s.Log("Checking for battery info in sysfs")
	out, err := d.Command("sh", "-c", "grep -ilH --color=no Battery /sys/class/power_supply/*/type").Output(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve battery info from sysfs: ", err)
	}
	paths := strings.Split(string(out), "\n")
	if len(paths) == 0 {
		s.Fatal("Failed to find any battery sysfs node.")
	}

	for _, path := range paths[:len(paths)-1] {
		s.Log("Found possible battery sysfs node: ", path)
		batteryName = batteryRegexp.FindStringSubmatch(path)[1]
		if pathExists(fmt.Sprintf(BATTERY_STATUS, batteryName)) &&
			pathExists(fmt.Sprintf(BATTERY_VOLTAGE_READING, batteryName)) &&
			pathExists(fmt.Sprintf(BATTERY_CURRENT_READING, batteryName)) {
			break
		}
	}
	if batteryName == "" {
		s.Fatal("Failed to find correct sysfs node!")
	}
	s.Log("Battery name is ", batteryName)
	batteryVoltage = fmt.Sprintf(BATTERY_VOLTAGE_READING, batteryName)
	batteryCurrent = fmt.Sprintf(BATTERY_CURRENT_READING, batteryName)

	s.Log("Checking if voltage from sysfs matches servo")
	servoVoltageReading, err := pxy.Servo().GetInt(ctx, "ppvar_vbat_mv")
	if err != nil {
		s.Fatal("Failed to read battery voltage from servo: ", err)
	}
	kernelVoltageReadingOut, err := d.Command("cat", batteryVoltage).Output(ctx)
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
	if math.Abs(float64(servoVoltageReading)-float64(kernelVoltageReading)) > VOLTAGE_MV_ERROR_MARGIN {
		s.Fatal(fmt.Sprintf(
			"Voltage reading from servo (%dmV) and kernel (%dmV) mistmatch beyond %dmV error margin",
			servoVoltageReading,
			kernelVoltageReading,
			VOLTAGE_MV_ERROR_MARGIN))
	} else {
		s.Log(fmt.Sprintf(
			"Voltage reading from servo (%dmV) and kernel (%dmV) do match within %dmV error margin",
			servoVoltageReading,
			kernelVoltageReading,
			VOLTAGE_MV_ERROR_MARGIN))
	}

	s.Log("Checking if current from sysfs matches servo")
	servoCurrentReading, err := pxy.Servo().GetInt(ctx, "ppvar_vbat_ma")
	if err != nil {
		s.Fatal("Failed to read battery current from servo: ", err)
	}
	kernelCurrentReadingOut, err := d.Command("cat", batteryCurrent).Output(ctx)
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
	if math.Abs(float64(servoCurrentReading)-float64(kernelCurrentReading)) > CURRENT_MA_ERROR_MARGIN {
		s.Fatal(fmt.Sprintf(
			"Current reading from servo (%dmA) and kernel (%dmA) mismatch beyond %dmA error margin",
			servoCurrentReading,
			kernelCurrentReading,
			CURRENT_MA_ERROR_MARGIN))
	} else {
		s.Log(fmt.Sprintf(
			"Current reading from servo (%dmA) and kernel (%dmA) do match within %dmA error margin",
			servoCurrentReading,
			kernelCurrentReading,
			CURRENT_MA_ERROR_MARGIN))
	}

	s.Log("Checking if battery temperature is reasonable")
	batteryTemperatureOut, err := pxy.Servo().RunECCommandGetOutput(ctx, "battery", []string{`Temp:.*\(([0-9.]+) C\)`})
	if err != nil {
		s.Fatal("Failed to read battery temperature from servo: ", err)
	}
	batteryTemperatureStr := batteryTemperatureOut[0].([]interface{})[1].(string)
	batteryTemperature, err := strconv.ParseFloat(batteryTemperatureStr, 64)
	if err != nil {
		s.Fatal(fmt.Sprintf("Failed to parse battery temperature from servo (%d) as float: ",
			batteryTemperatureStr,
			err))
	}
	s.Log("Battery temperature: ", batteryTemperature, " C")
	if batteryTemperature > BATTERY_TEMP_UPPER_BOUND || batteryTemperature < BATTERY_TEMP_LOWER_BOUND {
		s.Fatal(fmt.Sprintf(
			"Abnormal battery temperature %d (should be within %d-%d C)",
			batteryTemperature,
			BATTERY_TEMP_LOWER_BOUND,
			BATTERY_TEMP_UPPER_BOUND))
	} else {
		s.Log(fmt.Sprintf("Battery temperature %d C seems to be normal, within the %d-%d C limit",
			batteryTemperature,
			BATTERY_TEMP_LOWER_BOUND,
			BATTERY_TEMP_UPPER_BOUND))
	}

	if err = pxy.Servo().RunECCommand(ctx, "chan 0xffffffff"); err != nil {
		s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
	}
}
