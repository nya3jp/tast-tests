// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const gameplay_emulation string = "etr_keyboard_emulation"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtremeTuxRacer,
		Desc:         "Tests ExtremeTuxRacer gameplay in Crostini",
		Contacts:     []string{"shane.mckee@intel.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      10 * time.Minute,
		Pre:          crostini.StartedByDownload(),
		SoftwareDeps: []string{"chrome", "vm_host"},
		Data: []string{
			gameplay_emulation,
		},
	})
}

// The ExtremeTuxRacer test installs and runs ExtremeTuxRacer, using the emulator for keyboard emulation.
func ExtremeTuxRacer(ctx context.Context, s *testing.State) {
	RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}

func composeKeyboardEmulationString(s *testing.State, keyboardDevice string, emulationFile string) string {
	emulationString := fmt.Sprintf(
		"sleep 3 && evemu-play --insert-slot0 %s < %s && sleep 10",
		keyboardDevice,
		s.DataPath(emulationFile),
	)

	return emulationString
}

// Performs one round of ExtremeTuxRacer gameplay
func etrGameplayOnce(
	ctx context.Context,
	s *testing.State,
	cont *vm.Container,
	needsInitialization bool,
	keyboardDevice string,
) map[string]float64 {

	keyboardEmulationCmd := composeKeyboardEmulationString(s, keyboardDevice, gameplay_emulation)

	// Prevents the CPU usage measurements from being affected by any previous tests.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("ERROR: Failed to idle: ", err)
	}

	// Setting up to collect CPU usage
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Error("ERROR: Failed to setup benchmark mode:", err)
	}
	defer cleanUpBenchmark(ctx)

	var wg sync.WaitGroup
	measurements := make(map[string]float64)
	var userr error

	// Asynchronously measure CPU and power usage.
	wg.Add(1)
	go func() {
		defer wg.Done()
		measurements, userr = cpu.MeasureUsage(ctx, 90*time.Second)

		if userr != nil {
			s.Log("ERROR: failed to measure CPU and Power usage", userr)
		} else {
			s.Log("CPU and Power data: ", measurements)
		}
	}()

	// Asynchronously run the game and touchpad/keyboard emulation.
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Log("Running ExtremeTuxRacer")
		if err := cont.Command(ctx, "/usr/games/etr").Start(); err != nil {
			s.Fatal("ERROR: Failed to start ExtremeTuxRacer", err)
		}

		s.Log("Playing ExtremeTuxRacer keyboard emulation")
		cmd := testexec.CommandContext(ctx, "sh", "-c", keyboardEmulationCmd)
		if err := cmd.Run(); err != nil {
			s.Fatal("ERROR: Failed to run the ExtremeTuxRacer gameplay keyboard emulation file: ", err)
			cmd.DumpLog(ctx)
		}
	}()

	wg.Wait()

	return measurements
}

func findInputDevice(s *testing.State, deviceToFind string) string {

	deviceNum := 0
	device := ""

	// Look through /sys/class/input devices for keyboard to emulate
	for deviceFound := false; !deviceFound; {
		deviceNameFileString := fmt.Sprintf("/sys/class/input/event%d/device/name", deviceNum)
		s.Log("Checking for: ", deviceNameFileString)
		deviceNameFile, err := os.Open(deviceNameFileString)
		if err != nil {
			if !deviceFound {
				s.Fatal("ERROR: Could not locate a device to emulate: ", deviceToFind)
			}
		}

		deviceName, err := ioutil.ReadAll(deviceNameFile)
		if err != nil {
			s.Fatal("ERROR: Failed to read device file: ", err)
		}

		if strings.Contains(string(deviceName), deviceToFind) {
			deviceFound = true
			device = fmt.Sprintf("/dev/input/event%d", deviceNum)
		}

		deviceNum += 1
	}

	return device
}

// Measures CPU and and power consumed while running a basic game
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container) {

	const numberOfIterations int = 5
	keyboardDevice := findInputDevice(s, "keyboard")

	s.Log("Installing required packages to run ExtremeTuxRacer")
	if err := cont.Command(ctx, "sudo", "apt-get", "install", "-y", "extremetuxracer").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("ERROR: Failed to install packages: ", err)
	}

	cpuTotal := 0.0
	powerTotal := 0.0

	// Gameplay loop
	i := 0
	for numberOfIterations > i {
		oneLoopMeasurements := etrGameplayOnce(ctx, s, cont, i == 0, keyboardDevice)
		cpuTotal += oneLoopMeasurements["cpu"]
		powerTotal += oneLoopMeasurements["power"]

		i += 1
	}

	avgCPU := cpuTotal / float64(numberOfIterations)
	avgPower := powerTotal / float64(numberOfIterations)

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, avgCPU)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average cpu due to: ", err)
	}

	pv.Set(perf.Metric{
		Name:      "power_used",
		Unit:      "Watt",
		Direction: perf.SmallerIsBetter,
	}, avgPower)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average power due to: ", err)
	}
}
