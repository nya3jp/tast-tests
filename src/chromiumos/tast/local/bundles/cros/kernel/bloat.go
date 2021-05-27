// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

var (
	serialRe = regexp.MustCompile("^serial[0-9]+$")
)

const sysBus = "/sys/bus/"
const kallsymsPath = "/proc/kallsyms"

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

// Bloat finds kernel bloat and highlights areas that we should look at improving.
func Bloat(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	defer pv.Save(s.OutDir())

	findUnusedDriversAndDevices(ctx, s, pv)
	measureKernelText(ctx, s, pv)
}

// findUnusedDriversAndDevices find modules, drivers, and sometimes even
// devices, that are present on the DUT but aren't used. These things waste
// system memory and disk space so we should try to remove them.
func findUnusedDriversAndDevices(ctx context.Context, s *testing.State, pv *perf.Values) {
	driverCount := 0
	unusedDriverCount := 0
	unusedDeviceCount := 0
	unusedModuleDriverCount := 0

	unusedDriversFile, err := os.Create(filepath.Join(s.OutDir(), "unused_drivers.txt"))
	if err != nil {
		s.Fatal("Failed to create unused drivers log: ", err)
	}
	defer unusedDriversFile.Close()

	unusedDevicesFile, err := os.Create(filepath.Join(s.OutDir(), "unused_devices.txt"))
	if err != nil {
		s.Fatal("Failed to create unused devices log: ", err)
	}
	defer unusedDevicesFile.Close()

	unusedModuleDriversFile, err := os.Create(filepath.Join(s.OutDir(), "unused_module_drivers.txt"))
	if err != nil {
		s.Fatal("Failed to create unused module drivers log: ", err)
	}
	defer unusedModuleDriversFile.Close()

	busses, err := ioutil.ReadDir(sysBus)
	if err != nil {
		s.Fatalf("Failed to read %s for busses: %s", sysBus, err)
	}

	for _, bus := range busses {
		busName := bus.Name()
		driversDir := filepath.Join(sysBus, busName, "drivers/")
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

			if !hasDevice {
				if fromModule {
					fmt.Fprintln(unusedModuleDriversFile, driverDir)
					unusedModuleDriverCount++
				} else {
					fmt.Fprintln(unusedDriversFile, driverDir)
					unusedDriverCount++
				}
			}
			driverCount++
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
			"gpio",
			"iio",
			"soc",
			"workqueue":
			continue
		}

		devicesDir := filepath.Join(sysBus, busName, "devices/")
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
			if busName == "serial" && serialRe.MatchString(deviceName) {
				continue
			}
			if busName == "spmi" && strings.HasPrefix(deviceName, "spmi-") {
				continue
			}

			deviceDir := filepath.Join(devicesDir, deviceName)

			// Ignore PCI devices that aren't enabled as a driver can't attach
			if busName == "pci" {
				if b, err := ioutil.ReadFile(filepath.Join(deviceDir, "enable")); err == nil {
					if enabled, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64); err == nil {
						if enabled == 0 {
							continue
						}
					}
				}
			}

			deviceDriverDir := filepath.Join(deviceDir, "driver")

			_, err := os.Stat(deviceDriverDir)
			if err != nil {
				fmt.Fprintln(unusedDevicesFile, deviceDir)
				unusedDeviceCount++
			}
		}
	}

	pv.Set(
		perf.Metric{
			Name:      "kernel_unused_drivers",
			Unit:      "drivers",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		},
		float64(unusedDriverCount))
	pv.Set(
		perf.Metric{
			Name:      "kernel_unused_devices",
			Unit:      "devices",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		},
		float64(unusedDeviceCount))
	pv.Set(
		perf.Metric{
			Name:      "kernel_unused_module_drivers",
			Unit:      "drivers",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		},
		float64(unusedModuleDriverCount))
	pv.Set(
		perf.Metric{
			Name:      "kernel_drivers",
			Unit:      "drivers",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		},
		float64(driverCount))
}

// measureKernelText measures the number of bytes for the kernel text section
func measureKernelText(ctx context.Context, s *testing.State, pv *perf.Values) {
	startText := uint64(0)
	endText := uint64(0)

	kallsyms, err := os.Open(kallsymsPath)
	if err != nil {
		s.Fatal(err, "failed to open kallsyms: ", err)
	}
	defer kallsyms.Close()

	scanner := bufio.NewScanner(kallsyms)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.SplitN(line, " ", 3)
		if len(tokens) != 3 {
			s.Fatalf("Expected 3 tokens in %q, but got %d", line, len(tokens))
		}

		addr := tokens[0]
		symbol := tokens[2]
		if symbol == "_stext" {
			startText, err = strconv.ParseUint("0x"+addr, 0, 64)
			if err != nil {
				s.Fatal("Can't convert _stext to integer: ", err)
			}
		} else if symbol == "_etext" {
			endText, err = strconv.ParseUint("0x"+addr, 0, 64)
			if err != nil {
				s.Fatal("Can't convert _etext to integer: ", err)
			}
		}
	}

	if startText == 0 && endText == 0 {
		s.Error("Couldn't find _stext/_etext to measure kernel text size")
	} else {
		pv.Set(
			perf.Metric{
				Name:      "kernel_text",
				Unit:      "bytes",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			float64(endText-startText))
	}
}
