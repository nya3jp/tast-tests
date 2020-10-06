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
	"sync"
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
	suspendBlockTimeout   = 10 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullQualificationStress,
		Desc:         "Performs a full version of storage qualification test",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         stress.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Vars:         []string{"tast_disk_size_gb", "storage.slcQual"},
		Params: []testing.Param{{
			Name:    "setup_benchmarks",
			Val:     setupBenchmarks,
			Timeout: 1 * time.Hour,
		}, {
			Name:    "stress",
			Val:     stressRunner,
			Timeout: 5 * time.Hour,
		}, {
			Name:    "teardown_benchmarks",
			Val:     setupBenchmarks,
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

	actualSizeGb, err := info.SizeInGB()
	if err != nil {
		s.Fatal("Error selecting main storage device: ", err)
	}

	// Check if the requested disk size is within 10% of the actual.
	// Threshold is needed because we want to treat 512GB and 500GB as the same size.
	if int(math.Abs(float64(actualSizeGb-requestedSizeGb))) > actualSizeGb/10 {
		s.Fatalf("Requested disk size %dGB doesn't correspond to to the actual size %dGB",
			requestedSizeGb, actualSizeGb)
	}

	if dualQual(ctx, s) {
		if _, err := info.SlcDevice(); err != nil {
			s.Fatal("Dual qual is specified but SLC device is not present: ", err)
		}
	}
}

// setupBenchmarks captures and records bandwidth and latency disk benchmarks at the
// beginning and the end of the test suite.
func setupBenchmarks(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{ResultWriter: rw}

	// Run tests to collect metrics for boot device.
	runFioStressForBootDev(ctx, s, testConfig.WithJob("seq_write"))
	runFioStressForBootDev(ctx, s, testConfig.WithJob("seq_read"))
	runFioStressForBootDev(ctx, s, testConfig.WithJob("4k_write"))
	runFioStressForBootDev(ctx, s, testConfig.WithJob("4k_read"))
	runFioStressForBootDev(ctx, s, testConfig.WithJob("16k_write"))
	runFioStressForBootDev(ctx, s, testConfig.WithJob("16k_read"))

	if !dualQual(ctx, s) {
		return
	}
	// Run tests to collect metrics for Slc device.
	runFioStressForSlcDev(ctx, s, testConfig.WithJob("4k_write"))
	runFioStressForSlcDev(ctx, s, testConfig.WithJob("4k_read"))
}

// soakTestBlock runs long, write-intensive storage stresses.
func soakTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	testConfigNoVerify := &stress.TestConfig{}
	testConfigVerify := &stress.TestConfig{
		VerifyOnly:   true,
		ResultWriter: rw,
		Duration:     soakBlockTimeout / 2,
	}

	stressTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStressForBootDev(ctx, s, testConfigNoVerify.WithJob("64k_stress"))
			runFioStressForBootDev(ctx, s, testConfigVerify.WithJob("surfing"))
		},
	}

	if dualQual(ctx, s) {
		stressTask = append(stressTask,
			func(ctx context.Context) {
				runFioStressForSlcDev(ctx, s, testConfigNoVerify.WithJob("4k_write"))
				runFioStressForSlcDev(ctx, s, testConfigVerify.WithJob("4k_write"))
			})
	}

	runTasksInParallel(ctx, soakBlockTimeout, stressTasks)
}

// retentionTestBlock reads and then validates the same data after multiple short suspend cycles.
func retentionTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	writeConfig := stress.TestConfig{
		Job:      "8k_async_randwrite",
		Duration: retentionBlockTimeout,
	}
	// Verify disk consistency written by the initial FIO test.
	verifyConfig := stress.TestConfig{
		Job:        "8k_async_randwrite",
		VerifyOnly: true,
	}

	writeTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStressForBootDev(ctx, s, writeConfig)
		},
	}
	verifyTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStressForBootDev(ctx, s, verifyConfig)
		},
	}

	if dualQual(ctx, s) {
		writeTasks = append(writeTasks,
			func(ctx context.Context) {
				runFioStressForSlcDev(ctx, s, writeConfig)
			})
		verifyTasks = append(verifyTasks,
			func(ctx context.Context) {
				runFioStressForSlcDev(ctx, s, verifyConfig)
			})
	}

	runTasksInParallel(ctx, retentionBlockTimeout, writeTasks)

	// Run Suspend repeatedly until the timeout.
	pollOptions := &testing.PollOptions{
		Timeout:  retentionBlockTimeout,
		Interval: 30 * time.Second,
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := stress.Suspend(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to suspend DUT"))
		}
		return errors.New("retention test is still running normally")
	}, pollOptions); err != nil && !errors.As(err, &context.DeadlineExceeded) {
		s.Fatal("Failed running retention block: ", err)
	}
	runTasksInParallel(ctx, retentionBlockTimeout, verifyTasks)
}

// runContinuousStorageStress is a storage stress that is periodically interrupted by a power suspend.
func runContinuousStorageStress(ctx context.Context, s *testing.State, job string, rw *stress.FioResultWriter) {
	testConfig := stress.TestConfig{
		Job:          job,
		ResultWriter: rw,
	}
	// Running write stress continuously, until timeout.
	for {
		if err := runFioStressForBootDev(ctx, s, testConfig); err != nil && errors.As(err, &context.DeadlineExceeded) {
			return // Timeout exceeded.
		}
	}
}

// runContinuousSlcStorageStress is an slc storage stress that is periodically interrupted by a power suspend.
func runContinuousSlcStorageStress(ctx context.Context, s *testing.State, job string, rw *stress.FioResultWriter) {
	testConfig := stress.TestConfig{
		Job:          job,
		ResultWriter: rw,
	}
	// Running write stress continuously, until timeout.
	for {
		if err := runFioStressForSlcDev(ctx, s, testConfig); err != nil && errors.As(err, &context.DeadlineExceeded) {
			return // Timeout exceeded.
		}
	}
}

// runPeriodicPowerSuspend repeatedly suspends the DUT that is running a FIO stress.
// Exits only when context deadline is exceeded.
func runPeriodicPowerSuspend(ctx context.Context) {
	// Indefinite loop of randomized sleeps and power suspends.
	for {
		sleepDuration := time.Duration(rand.Intn(30)+30) * time.Second
		if err := testing.Sleep(ctx, sleepDuration); errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err := stress.Suspend(ctx); err != nil && errors.As(err, &context.DeadlineExceeded) {
			return
		}
	}
}

// suspendTestBlock triggers periodic power suspends while running disk stress.
// This test block doesn't validate consistency nor status of the disk stress, which
// is done by measuring storage degradation by the next soak iteration.
func suspendTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter) {
	if deadline, _ := ctx.Deadline(); time.Until(deadline) < suspendBlockTimeout {
		s.Fatal("Context timeout occurs before suspend block timeout")
	}

	tasks := []func(context.Context){
		func(ctx context.Context) {
			runContinuousStorageStress(ctx, s, "write_stress", rw)
		},
		runPeriodicPowerSuspend,
	}

	if dualQual(ctx, s) {
		tasks = append(tasks,
			func(ctx context.Context) {
				runContinuousSlcStorageStress(ctx, s, "4k_write", rw)
			})
	}

	runTasksInParallel(ctx, suspendBlockTimeout, tasks)
}

// dualQual checks if the test shall be run as dual-namespace AVL.
func dualQual(ctx context.Context, s *testing.State) bool {
	if val, ok := s.Var("storage.slcQual"); ok {
		dual, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Cannot parse argumet 'storage.QuickStress.slcQual' of type bool: ", err)
		}
		return dual
	}
	return false
}

// slcDevice returns an Slc device path for dual-namespace AVL.
func slcDevice(ctx context.Context, s *testing.State) string {
	info, err := stress.ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}
	slc, err := info.SlcDevice()
	if slc == nil {
		s.Fatal("Dual qual is specified but SLC device is not present: ", err)
	}
	return filepath.Join("/dev/", slc.Name)
}

// runTasksInParallel runs stress tasks in parallel.
// Returns true if one or more tasks timed out.
func runTasksInParallel(ctx context.Context, timeout time.Duration, tasks []func(ctx context.Context)) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	testing.ContextLog(ctx, "Starting parallel tasks at: ", time.Now())

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(taskToRun func(ctx context.Context)) {
			taskToRun(ctx)
			wg.Done()
		}(task)
	}
	wg.Wait()
	testing.ContextLog(ctx, "Finished parallel tasks at: ", time.Now())

	return false
}

// writeTestStatusFile writes test status JSON file to test's output folder.
// Status file contains start/end times and final test status (passed/failed).
func writeTestStatusFile(ctx context.Context, outDir string, passed bool, startTimestamp time.Time) error {
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
	filename := filepath.Join(outDir, "status.json")
	if err := ioutil.WriteFile(filename, file, 0644); err != nil {
		return errors.Wrap(err, "failed saving test status to file")
	}
	return nil
}

// runFioStressForBootDev runs an fio job for the boot device.
// If fio returns an error, this function will fail the Tast test.
func runFioStressForBootDev(ctx context.Context, s *testing.State, testConfig stress.TestConfig) error {
	config := testConfig.WithPath(stress.BootDeviceFioPath).WithJobFile(s.DataPath(testConfig.Job))

	if err := stress.RunFioStress(ctx, config); err != nil {
		if errors.As(err, &context.DeadlineExceeded) {
			return err
		}
		s.Fatal("FIO stress failed: ", err)
	}
	return nil
}

// runFioStressForSlcDev runs an fio job if slc namespace is present, otherwise
// returns immediately
// If fio returns an error, this function will fail the Tast test.
func runFioStressForSlcDev(ctx context.Context, s *testing.State, testConfig stress.TestConfig) error {
	config := testConfig.WithPath(slcDevice(ctx, s)).WithJobFile(s.DataPath(testConfig.Job))
	if err := stress.RunFioStress(ctx, config); err != nil {
		if errors.As(err, &context.DeadlineExceeded) {
			return err
		}
		s.Fatal("FIO stress failed: ", err)
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
	subtest := s.Param().(func(context.Context, *testing.State, *stress.FioResultWriter))
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
	if err := writeTestStatusFile(ctx, s.OutDir(), passed, start); err != nil {
		s.Fatal("Error writing status file: ", err)
	}
}
