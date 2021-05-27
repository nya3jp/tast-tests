// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Bloat,
		Desc: "Tracks kernel bloat",
		Contacts: []string{
			"swboyd@chromium.org",
			"chromeos-kernel-test@google.com",
		},
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
	})
}

// Bloat finds modules, drivers, and sometimes even devices, that are present
// on the DUT but aren't used. These things waste memory so we should try
// to remove them.
func Bloat(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	defer pv.Save(s.OutDir())

	findUnusedDriversAndDevices(ctx, s, pv)
}

func findUnusedDriversAndDevices(ctx context.Context, s *testing.State, pv *perf.Values) {
	unusedDriverCount := 0
	unusedDeviceCount := 0
	unusedDriversFile, err := os.Create(filepath.Join(s.OutDir(), "unused_drivers.txt"))
	if err != nil {
		s.Fatal("Failed to create unused drivers log: ", err)
	}
	defer unusedDriversFile.Close()

	unusedDevicesFile, err := os.Create(filepath.Join(s.OutDir(), "unused_devices.txt"))
	if err != nil {
		s.Fatal("Failed to create unused drivers log: ", err)
	}
	defer unusedDevicesFile.Close()

	busses, err := ioutil.ReadDir("/sys/bus/")
	if err != nil {
		s.Fatal("Failed to read /sys/bus for busses: ", err)
	}

	for _, bus := range busses {
		busName := bus.Name()
		driversDir := filepath.Join("/sys/bus/", busName, "drivers/")
		drivers, err := ioutil.ReadDir(driversDir)
		if err != nil {
			s.Fatalf("Failed to find drivers for bus %s: %s", busName, err)
		}

		for _, driver := range drivers {
			driverName := driver.Name()
			driverDir := filepath.Join(driversDir, driverName)
			files, err := ioutil.ReadDir(driverDir)
			if err != nil {
				s.Fatalf("Failed to read driver directory for %s: %s", driverName, err)
			}

			hasDevice := false
			fromModule := false
			for _, file := range files {
				fileName := file.Name()
				if fileName == "module" {
					fromModule = true
					continue
				}

				// Assume it's a device symlink
				if file.Mode()&os.ModeSymlink == os.ModeSymlink {
					hasDevice = true
				}
			}

			if !hasDevice && !fromModule {
				fmt.Fprintln(unusedDriversFile, driverDir)
				unusedDriverCount++
			}
		}

		// Ignore busses that only ever have devices, not drivers
		switch busName {
		case
			"clockevents",
			"clocksource",
			"coresight",
			"cpu",
			"event_source",
			"genpd",
			"soc",
			"workqueue":
			continue
		}

		devicesDir := filepath.Join("/sys/bus/", busName, "devices/")
		devices, err := ioutil.ReadDir(devicesDir)
		if err != nil {
			s.Fatalf("Failed to find devices for bus %s: %s", busName, err)
		}

		for _, device := range devices {
			deviceName := device.Name()

			// Ignore devices on busses that are "containers"
			if busName == "i2c" && strings.HasPrefix(deviceName, "i2c-") {
				continue
			}
			if busName == "spmi" && strings.HasPrefix(deviceName, "spmi-") {
				continue
			}

			deviceDir := filepath.Join(devicesDir, deviceName)
			deviceDriverDir := filepath.Join(deviceDir, "driver")

			_, err := os.Stat(deviceDriverDir)
			if err != nil {
				fmt.Fprintln(unusedDevicesFile, deviceDir)
				unusedDeviceCount++
			}
		}
	}

	pv.Append(
		perf.Metric{
			Name:      "kernel_unused_drivers",
			Unit:      "drivers",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		},
		float64(unusedDriverCount))

	pv.Append(
		perf.Metric{
			Name:      "kernel_unused_devices",
			Unit:      "devices",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		},
		float64(unusedDeviceCount))
}
