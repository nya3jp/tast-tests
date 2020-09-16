// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/storage/stress"
	"chromiumos/tast/testing"
)

// subTestFunc is the code associated with a sub-test.
type subTestFunc func(context.Context, *testing.State, *stress.FioResultWriter)

const (
	// Main storage device has to be >= 16GB.
	mainStorageDeviceMinSize = 16 * 1024 * 1024 * 1024

	// Max number of retries for a sub-test of a universal test block.
	maxSubtestRetry = 3

	// Total duration of the soak, retention and suspend sub-tests.
	soakBlockTimeout      = 2 * time.Hour
	retentionBlockTimeout = 20 * time.Minute
	suspendBlockTimeout   = 20 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullQualificationStress,
		Desc:         "Performs a full version of storage qualification test",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         stress.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Vars:         []string{"tast_disk_size_gb"},
		Params: []testing.Param{{
			Name:    "00_setup_benchmarks",
			Val:     subTestFunc(setupBenchmarks),
			Timeout: 1 * time.Hour,
		}, {
			Name:    "01_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "02_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "03_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "04_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "05_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "06_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "07_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "08_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "09_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "10_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "11_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "12_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "13_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "14_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "15_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "16_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "17_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "18_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "19_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "20_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "21_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "22_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "23_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "24_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "25_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "26_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "27_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "28_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "29_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "30_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "31_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "32_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "33_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "34_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "35_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "36_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "37_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "38_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "39_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "40_stress",
			Val:     subTestFunc(stressRunner),
			Timeout: 5 * time.Hour,
		}, {
			Name:    "99_teardown_benchmarks",
			Val:     subTestFunc(setupBenchmarks),
			Timeout: 1 * time.Hour,
		}},
	})
}

func setupChecks(ctx context.Context, s *testing.State) {
	// Fetching info of all storage devices.
	info, err := stress.ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}

	// Checking the size of the main storage device.
	if err := info.CheckMainDeviceSize(mainStorageDeviceMinSize); err != nil {
		s.Fatal("Main storage disk is too small: ", err)
	}

	// Save storage info to results.
	if err := info.SaveDiskInfo(filepath.Join(s.OutDir(), "diskinfo.json")); err != nil {
		s.Fatal("Error saving disk info: ", err)
	}

	// Get the user-requested and actual disk size.
	varStr := s.RequiredVar("tast_disk_size_gb")
	requestedSizeGb, err := strconv.Atoi(varStr)
	if err != nil {
		s.Fatal("Bad format of request disk size: ", err)
	}

	actualSizeGb, err := info.SizeInGb()
	if err != nil {
		s.Fatal("Error selecting main storage device: ", err)
	}

	// Check if the requested disk size is within 10% of the actual.
	// Threshold is needed because we want to treat 512GB and 500GB as the same size.
	if int(math.Abs(float64(actualSizeGb-requestedSizeGb))) > actualSizeGb/10 {
		s.Fatalf("Requested disk size %dGB doesn't correspond to to the actual size %dGB",
			requestedSizeGb, actualSizeGb)
	}
}

// setupBenchmarks captures and records bandwidth and latency disk benchmarks at the
// beginning and the end of the test suite.
func setupBenchmarks(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	// Run tests to collect metrics.
	testConfig := &stress.TestConfig{Path: stress.BootDeviceFioPath, ResultWriter: rw}
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("seq_write"))
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("seq_read"))
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("4k_write"))
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("4k_read"))
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("16k_write"))
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("16k_read"))
}

// soakTestBlock runs long, write-intensive storage stresses.
func soakTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	testConfigNoVerify := &stress.TestConfig{Path: stress.BootDeviceFioPath}
	testConfigVerify := &stress.TestConfig{
		Path:         stress.BootDeviceFioPath,
		VerifyOnly:   true,
		ResultWriter: rw,
	}

	stress.RunFioStressCritical(ctx, s,
		testConfigNoVerify.WithJob("64k_stress").WithDuration(soakBlockTimeout/2))
	stress.RunFioStressCritical(ctx, s,
		testConfigVerify.WithJob("surfing").WithDuration(soakBlockTimeout/2))
}

// retentionTestBlock reads and then validates the same data after multiple short suspend cycles.
func retentionTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{Path: stress.BootDeviceFioPath}
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("8k_async_randwrite").WithDuration(retentionBlockTimeout))
	// Rounding start time is needed to strip monotonic clock, which is skewed by sleeps.
	for start := time.Now().Round(0); time.Since(start) < retentionBlockTimeout; {
		if err := stress.Suspend(ctx); err != nil {
			s.Fatal("Failed suspeding the DUT: ", err)
		}
		// Delay between subsequent suspends to allow SSD to fully wake up.
		testing.Sleep(ctx, 30*time.Second)
	}
	stress.RunFioStressCritical(ctx, s, testConfig.WithJob("8k_async_randwrite").WithVerifyOnly(true))
}

// suspendStorageStress is a storage stress that runs during DUT suspend test.
func suspendStorageStress(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{Path: stress.BootDeviceFioPath}
	// Running write stress continuously, until timeout.
	for {
		stress.RunFioStress(ctx, s, testConfig.WithJob("write_stress").WithResultWriter(rw))
	}
}

// suspendPowerSuspend repeatedly suspends the DUT that is running a FIO stress.
func suspendPowerSuspend(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	// Indefinite loop of randomized sleeps and power suspends.
	for {
		testing.Sleep(ctx, time.Duration(rand.Intn(30)+30)*time.Second)
		stress.Suspend(ctx)
	}
}

func suspendTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	runParallelSubTests(ctx, s, rw, suspendBlockTimeout, []subTestFunc{
		subTestFunc(suspendStorageStress),
		subTestFunc(suspendPowerSuspend),
	})
}

// runParallelSubTests runs result producing blocks in parallel.
// Returns true if tests timed out.
func runParallelSubTests(ctx context.Context, s *testing.State, rw *stress.FioResultWriter,
	timeout time.Duration, subTests []subTestFunc) bool {

	done := make(chan bool)
	// Starting all subtests in parallel.
	for _, subTest := range subTests {
		go func(subTest subTestFunc) {
			s.Log("Starting subtest: ", subTest)
			subTest(ctx, s, rw)
			done <- true
		}(subTest)
	}

	// Subtests are normally expected to run until timeout.
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-done:
			s.Log("A subtest of a parallel test has finished")
		case <-timeoutChan:
			s.Log("Parallel test timed out")
			return true
		}
	}
}

// writeTestStatusFile writes test status JSON file to test's output folder.
// Status file contains start/end times and final test status (passed/failed).
func writeTestStatusFile(ctx context.Context, s *testing.State, passed bool, startTimestamp time.Time) error {
	statusFileStruct := struct {
		Started  string `json:"started"`
		Finished string `json:"finished"`
		Passed   bool   `json:"passed"`
	}{
		Started:  startTimestamp.Format(time.RFC3339),
		Finished: time.Now().Format(time.RFC3339),
		Passed:   passed,
	}

	file, err := json.MarshalIndent(statusFileStruct, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling test status to JSON")
	}
	filename := filepath.Join(s.OutDir(), "status.json")
	err = ioutil.WriteFile(filename, file, 0644)
	if err != nil {
		return errors.Wrap(err, "failed saving test status to file")
	}
	return nil
}

// stressRunner is the main entry point of the unversal stress block.
// It runs all other functional sub-tests in a sequence, retrying failed sub-tests.
func stressRunner(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	for _, tc := range []struct {
		name     string
		function subTestFunc
	}{
		{
			name:     "soak",
			function: subTestFunc(soakTestBlock),
		},
		{
			name:     "suspend",
			function: subTestFunc(suspendTestBlock),
		},
		{
			name:     "retention",
			function: subTestFunc(retentionTestBlock),
		},
	} {
		for retries := 0; retries < maxSubtestRetry; retries++ {
			s.Logf("Subtest: %s, retry: %d of %d", tc.name, retries+1, maxSubtestRetry)
			passed := s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
				tc.function(ctx, s, rw)
			})
			if passed {
				break
			}
		}
	}
}

// FullQualificationStress runs a full version of disk IO qualification test.
// The full run of the test can take anything between 2-14 days.
func FullQualificationStress(ctx context.Context, s *testing.State) {
	subtest := s.Param().(subTestFunc)
	start := time.Now()

	// Before running any functional test block, test setup should be validated.
	passed := s.Run(ctx, "setup_checks", func(ctx context.Context, s *testing.State) {
		setupChecks(ctx, s)
	})
	if passed {
		passed = s.Run(ctx, "storage_subtest", func(ctx context.Context, s *testing.State) {
			resultWriter := &stress.FioResultWriter{}
			defer resultWriter.Save(ctx, s.OutDir())
			subtest(ctx, s, resultWriter)
		})
	}
	if err := writeTestStatusFile(ctx, s, passed, start); err != nil {
		s.Fatal("Error writing status file: ", err)
	}
}
