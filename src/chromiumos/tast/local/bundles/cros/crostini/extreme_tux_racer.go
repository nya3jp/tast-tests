// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"sync"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ExtremeTuxRacer,
		Desc:     "Tests ExtremeTuxRacer gameplay in Crostini",
		Contacts: []string{"shane.mckee@intel.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Params: []testing.Param{{
			Name:      "artifact",
			Pre:       crostini.StartedByArtifact(),
			ExtraData: []string{crostini.ImageArtifact},
			Timeout:   7 * time.Minute,
		}, {
			Name:    "download",
			Pre:     crostini.StartedByDownload(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// Performs one round of ExtremeTuxRacer gameplay
func etrGameplayOnce(ctx context.Context, s *testing.State, cont *vm.Container, needsInitialization bool) map[string]float64 {

	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}
	defer kb.Close()

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

		keys := []string{"\x1b", "\x1b", "\x1b"}
		for _, key := range keys {
			time.Sleep(100 * time.Millisecond)
			if err := kb.Accel(ctx, key); err != nil {
				s.Fatal("Keyboard playback failed.", err)
			}
		}
	}()

	// Asynchronously run the game and touchpad/keyboard emulation.
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Log("Running ExtremeTuxRacer")
		appID, exitCallback, err := crostini.LaunchGUIApp(ctx, tconn, cont.Command(ctx, "env", "GDK_BACKEND=wayland", "/usr/games/etr"))
		if err != nil {
			s.Fatal("Failed to launch crostini app: ", err)
		}
		defer exitCallback()
		s.Log("Launched crostini app with ID: ", appID)

		time.Sleep(3 * time.Second)
		keys := []string{"Tab", "Tab", "Enter", "Tab", "Enter", "Tab", "Tab", "Tab", "Tab", "Tab", "Tab", "Enter"}
		for _, key := range keys {
			if err := kb.Accel(ctx, key); err != nil {
				s.Fatal("Keyboard playback failed.", err)
			}
		}
	}()

	wg.Wait()

	return measurements
}

// Measures CPU and and power consumed while running a basic game
func ExtremeTuxRacer(ctx context.Context, s *testing.State) {

	const numberOfIterations int = 1
	cont := s.PreValue().(crostini.PreData).Container

	s.Log("Installing required packages to run ExtremeTuxRacer")
	if err := cont.Command(ctx, "sudo", "apt-get", "install", "-y", "extremetuxracer").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("ERROR: Failed to install packages: ", err)
	}

	cpuTotal := 0.0
	powerTotal := 0.0

	// Gameplay loop
	i := 0
	for numberOfIterations > i {
		oneLoopMeasurements := etrGameplayOnce(ctx, s, cont, i == 0)
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
