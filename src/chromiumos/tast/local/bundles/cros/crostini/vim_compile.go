// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"regexp"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VimCompile,
		Desc:         "Crostini performance test which compiles vim",
		Contacts:     []string{"sushma.venkatesh.reddy@intel.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      12 * time.Minute,
		Pre:          crostini.StartedByDownload(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

var isIntelDevice bool // isIntelDevice variable is used to make a decision on power measurement.

// VimCompile downloads the VM image from the staging bucket, i.e. it emulates the setup-flow that a user has.
// It compiles vim multiple times and captures the average amount of time taken to compile it.
func VimCompile(ctx context.Context, s *testing.State) {
	const (
		numberOfIterations = 5 // numberOfIterations is set to the number of times vim is to be compiled.
		configureVim       = "cd /home/testuser/vim/src && ./configure"
		makeVim            = "cd /home/testuser/vim/src && make -j > /dev/null"
		removeVim          = "cd /home/testuser && rm -rf vim"
		untarVim           = "cd /home/testuser && tar -xvf vim.tar.gz"
		raplExec           = "/usr/bin/dump_intel_rapl_consumption"
	)
	cont := s.PreValue().(crostini.PreData).Container
	var collectCPU, collectPower, powerConsumption float64
	var collectTime, endTime time.Duration
	var wg sync.WaitGroup
	compileComplete := make(chan bool)
	i := 0

	setupTest(ctx, s, cont)

	s.Logf("Compiling vim %d times", numberOfIterations)
	for i < numberOfIterations {
		// Configuring vim package.
		s.Log("Configuring vim")
		if err := executeShellCommand(ctx, cont, configureVim); err != nil {
			s.Error("Failed to configure vim package: ", err)
		}

		// Compiling vim package.
		s.Logf("Running make vim (iteration %d)", i+1)

		// Prevents the CPU usage measurements from being affected by any previous tests.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to idle: ", err)
		}

		cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
		if err != nil {
			s.Fatal("Failed to set up benchmark mode: ", err)
		}
		defer cleanUpBenchmark(ctx)

		// Capture initial stats across all CPUs as reported by /proc/stat.
		statBegin, err := cpu.MeasureCPUStart(ctx)
		if err != nil {
			s.Log("Failed to get CPU status: ", err)
		}

		// Power consumption is only measured on Intel devices that support the
		// dump_intel_rapl_consumption command.
		if isIntelDevice {
			// Measures power consumption asynchronously.
			wg.Add(1)
			var collectraplPower float64
			go func() {
				defer wg.Done()
				for j := 0; ; j++ {
					select {
					case <-compileComplete:
						//  Avg power consumption (in Watts) by reading the RAPL 'pkg' entry,
						//  which gives a measure of the total SoC power consumption.
						powerConsumption = collectraplPower / float64(j)
						return
					default:
						cmd := testexec.CommandContext(ctx, raplExec)
						powerConsumptionOutput, err := cmd.CombinedOutput()
						if err != nil {
							s.Log("Unable to print rapl data: ", err)
							return
						}
						var powerConsumptionRegex = regexp.MustCompile(`(\d+\.\d+)`)
						match := powerConsumptionRegex.FindAllString(string(powerConsumptionOutput), 1)
						if len(match) != 1 {
							s.Logf("failed to parse output of %s", raplExec)
						}
						raplPower, powerErr := strconv.ParseFloat(match[0], 64)
						if powerErr != nil {
							s.Error("Failed to measure Power consumption: ", powerErr)
						}
						collectraplPower += raplPower
						s.Log("Pkg power in watts: ", raplPower)
					}
				}
			}()
		}

		// Measures time taken to compile vim asynchronously.
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Log("Measuring compile time")
			start := time.Now()
			makeerr := executeShellCommand(ctx, cont, makeVim)
			if makeerr != nil {
				s.Fatal("Failed to make vim package: ", makeerr)
			}
			endTime = time.Since(start)
			if isIntelDevice {
				compileComplete <- true
			}
		}()

		wg.Wait()

		// Capture final stats across all CPUs as reported by /proc/stat
		// and calculate the CPU usage between start and final stats.
		cpuUsage, cpuErr := cpu.MeasureCPUEnd(ctx, statBegin)
		if cpuErr != nil {
			s.Fatal("Failed to measure CPU consumption: ", cpuErr)
		} else {
			s.Logf("Iteration %d, CPU usage: %fs", i+1, cpuUsage)
			collectCPU += cpuUsage
		}

		// Displaying compile time data.
		s.Logf("Iteration %d, Compile time: %fs", i+1, float64(endTime.Seconds()))
		collectTime += endTime

		if isIntelDevice {
			// Displaying power consumption data.
			s.Logf("Iteration %d, powerConsumption: %fs", i+1, powerConsumption)
			collectPower += powerConsumption
		}

		// Removing vim directory.
		s.Log("Removing vim directory")
		if err := executeShellCommand(ctx, cont, removeVim); err != nil {
			s.Fatal("Failed to remove vim package: ", err)
		}

		// Untaring vim packge for subsequent iterations.
		s.Log("Untaring vim package")
		if err := executeShellCommand(ctx, cont, untarVim); err != nil {
			s.Fatal("Failed to untar vim package: ", err)
		}
		i++
	}

	// Calculating average time to compile vim.
	avgTime := collectTime / numberOfIterations
	s.Logf("Average time to compile vim: %fs", float64(avgTime.Seconds()))

	// Saving data to Chrome Performance Dashboard - https://chromeperf.appspot.com/.
	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "compile_time",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, float64(avgTime.Seconds()))
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average compile time due to: ", err)
	}

	// Calculating average CPU usage to compile vim.
	avgCPU := collectCPU / numberOfIterations
	s.Logf("Average CPU usage to compile vim: %fs", avgCPU)

	pv.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, avgCPU)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average cpu time due to: ", err)
	}

	if isIntelDevice {
		// Calculating average power to compile vim.
		avgPower := collectPower / numberOfIterations
		s.Logf("Average power to compile vim: %fs", avgPower)

		pv.Set(perf.Metric{
			Name:      "power_used",
			Unit:      "Watt",
			Direction: perf.SmallerIsBetter,
		}, avgPower)
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save average watt consumption due to: ", err)
		}
	}
}

// setupTest sets up the device to compile vim.
func setupTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	const (
		shaValue       = "fbbd10" // shaValue is from vim github where we are checking out.
		installLibs    = "sudo apt-get install -y gcc make libncurses5-dev libncursesw5-dev"
		fixMissingLibs = "sudo apt-get update --fix-missing"
		cloneVim       = "git clone https://github.com/vim/vim.git"
		checkoutVimSha = "cd /home/testuser/vim/src && git checkout " + shaValue
		tarVim         = "tar -czvf vim.tar.gz vim"
		checkCPUType   = "lscpu | grep -c GenuineIntel"
	)

	s.Log("Installing required packages to compile vim")
	if err := executeShellCommand(ctx, cont, installLibs); err != nil {
		s.Fatal("Failed to install packages: ", err)
	}

	s.Log("Installing missing dependencies")
	if err := executeShellCommand(ctx, cont, fixMissingLibs); err != nil {
		s.Fatal("Failed to update dependencies: ", err)
	}

	s.Log("Cloning vim from github")
	if err := executeShellCommand(ctx, cont, cloneVim); err != nil {
		s.Fatal("Failed to clone vim package: ", err)
	}

	// This is required to ensure we measure the same content each time.
	s.Log("Checking out a specific tag in vim")
	if err := executeShellCommand(ctx, cont, checkoutVimSha); err != nil {
		s.Error("Failed to check out a specific tag due to: ", err)
	}

	s.Log("Compressing and saving the vim folder")
	if err := executeShellCommand(ctx, cont, tarVim); err != nil {
		s.Error("Failed to compress vim folder: ", err)
	}

	s.Log("Checking if it is an Intel processor")
	cpuCommand := testexec.CommandContext(ctx, "sh", "-c", checkCPUType)
	_, err := cpuCommand.CombinedOutput()
	if err != nil {
		s.Log("Unable to get CPU information: ", err)
		s.Log("rapl data will not be measured")
		isIntelDevice = false
	} else {
		s.Log("rapl data will be measured")
		isIntelDevice = true
	}
}

// executeShellCommand executes the shell commands on container.
func executeShellCommand(ctx context.Context, cont *vm.Container, cmd string) error {
	return cont.Command(ctx, "sh", "-c", cmd).Run(testexec.DumpLogOnError)
}
