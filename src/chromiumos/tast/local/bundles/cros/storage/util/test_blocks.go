// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// Main storage device has to be >= 16GB.
	mainStorageDeviceMinSize = 16 * 1024 * 1024 * 1024

	// Max number of retries for a sub-test of a universal test block.
	maxSubtestRetry = 3

	// DefaultRetentionBlockTimeout is the duration of the retention sub-test.
	DefaultRetentionBlockTimeout = 20 * time.Minute
	// DefaultSuspendBlockTimeout is the total duration of the suspend sub-test.
	DefaultSuspendBlockTimeout = 10 * time.Minute
)

// SetupBenchmarks captures and records bandwidth and latency disk benchmarks at the
// beginning and the end of the test suite.
func SetupBenchmarks(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	testConfig := &TestConfig{ResultWriter: rw}

	testing.ContextLog(ctx, "Test device: ", testParam.TestDevice)
	// Run tests to collect metrics for boot device.
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("seq_write"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("seq_read"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("4k_write"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("4k_write_qd4"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("4k_read_qd4"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("4k_read"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("16k_write"))
	runFioStress(ctx, s, testConfig.WithPath(testParam.TestDevice).WithJob("16k_read"))

	if testParam.IsSlcEnabled {
		// Run tests to collect metrics for Slc device.
		runFioStress(ctx, s, testConfig.WithPath(testParam.SlcDevice).WithJob("4k_write_qd4"))
		runFioStress(ctx, s, testConfig.WithPath(testParam.SlcDevice).WithJob("4k_read_qd4"))
	}
}

// soakTestBlock runs long, write-intensive storage stresses.
func soakTestBlock(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	const (
		slcTestDuration    = 120 * time.Minute
		stressTestDuration = 240 * time.Minute
	)

	testConfigNoVerify := &TestConfig{}
	testConfigVerify := &TestConfig{
		VerifyOnly:   true,
		ResultWriter: rw,
	}

	stressTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStress(ctx, s, testConfigNoVerify.WithPath(BootDeviceFioPath).WithJob("64k_stress").WithDuration(stressTestDuration))
			// NoVerify surf block to exercise device. Run once. Duration can be found in data/recovery
			runFioStress(ctx, s, testConfigNoVerify.WithPath(BootDeviceFioPath).WithJob("recovery"))
			// Verify surfing block for performance evaluation. Run once. Duration can be found in data/surfing
			runFioStress(ctx, s, testConfigVerify.WithPath(BootDeviceFioPath).WithJob("surfing"))
		},
	}

	if testParam.IsSlcEnabled {
		stressTasks = append(stressTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, testConfigNoVerify.WithPath(testParam.SlcDevice).WithJob("4k_write").WithDuration(slcTestDuration))
				runFioStress(ctx, s, testConfigVerify.WithPath(testParam.SlcDevice).WithJob("4k_write").WithDuration(slcTestDuration))
			})
	}

	runTasksInParallel(ctx, 0, stressTasks)
}

// retentionTestBlock reads and then validates the same data after multiple short suspend cycles.
func retentionTestBlock(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	writeConfig := TestConfig{
		Job:      "8k_async_randwrite",
		Duration: testParam.RetentionBlockTimeout,
	}
	// Verify disk consistency written by the initial FIO test.
	verifyConfig := TestConfig{
		Job:        "8k_async_randwrite",
		VerifyOnly: true,
	}

	writeTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStress(ctx, s, writeConfig.WithPath(BootDeviceFioPath))
		},
	}
	verifyTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStress(ctx, s, verifyConfig.WithPath(BootDeviceFioPath))
		},
	}

	if testParam.IsSlcEnabled {
		writeTasks = append(writeTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, writeConfig.WithPath(testParam.SlcDevice))
			})
		verifyTasks = append(verifyTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, verifyConfig.WithPath(testParam.SlcDevice))
			})
	}

	runTasksInParallel(ctx, 0, writeTasks)

	// Run Suspend repeatedly until the timeout.
	pollOptions := &testing.PollOptions{
		Timeout:  testParam.RetentionBlockTimeout,
		Interval: 30 * time.Second,
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := Suspend(ctx, testParam.SkipS0iXResidencyCheck); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to suspend DUT"))
		}
		return errors.New("retention test is still running normally")
	}, pollOptions); err != nil && !errors.As(err, &context.DeadlineExceeded) {
		s.Fatal("Failed running retention block: ", err)
	}
	runTasksInParallel(ctx, 0, verifyTasks)
}

// suspendTestBlock triggers periodic power suspends while running disk
// This test block doesn't validate consistency nor status of the disk stress, which
// is done by measuring storage degradation by the next soak iteration.
func suspendTestBlock(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	if deadline, _ := ctx.Deadline(); time.Until(deadline) < testParam.SuspendBlockTimeout {
		s.Fatal("Context timeout occurs before suspend block timeout")
	}

	tasks := []func(context.Context){
		func(ctx context.Context) {
			runContinuousStorageStress(ctx, "write_stress", s.DataPath("write_stress"), rw, testParam.TestDevice)
		},
		func(ctx context.Context) {
			runPeriodicPowerSuspend(ctx, testParam.SkipS0iXResidencyCheck)
		},
	}

	if testParam.IsSlcEnabled {
		tasks = append(tasks,
			func(ctx context.Context) {
				runContinuousStorageStress(ctx, "4k_write", s.DataPath("4k_write"), rw, testParam.SlcDevice)
			})
	}

	runTasksInParallel(ctx, testParam.SuspendBlockTimeout, tasks)
}

// trimTestBlock is a dispatcher function to start trim test on the boot device
// and on the slc.
func trimTestBlock(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	bootDevPartition, err := RootPartitionForTrim(ctx)
	if err != nil {
		s.Fatal("Failed to select partition for trim stress: ", err)
	}
	trimTestBlockImpl(ctx, s, bootDevPartition, rw)

	if testParam.IsSlcEnabled {
		trimTestBlockImpl(ctx, s, testParam.SlcDevice, rw)
	}
}

// trimTestBlockImpl performs data integrity trim test on an unmounted partition.
// This test will write 1 GB of data and verify that trimmed data are gone and untrimmed data are unaffected.
// The verification will be run in 5 passes with 0%, 25%, 50%, 75%, and 100% of data trimmed.
// Also, perform 4K random read QD32 before and after trim. We should see some speed / latency difference
// if the device firmware trim data properly.
func trimTestBlockImpl(ctx context.Context, s *testing.State, trimPath string, rw *FioResultWriter) {
	filesize, err := PartitionSize(ctx, trimPath)
	if err != nil {
		s.Fatal("Failed to acquire size for partition: ", err)
	}
	// Make file size multiple of 4 * chunk size to account for all passes,
	// i.e. 25% = 1/4, 75% = 3/4.
	filesize = filesize - filesize%(4*TrimChunkSize)
	s.Logf("Filename: %s, filesize: %d", trimPath, filesize)

	f, err := os.OpenFile(trimPath, os.O_RDWR, 0666)
	if err != nil {
		s.Fatal("Failed to open device: ", err)
	}
	defer f.Close()

	if err := RunTrim(f, 0, filesize); err != nil {
		s.Fatal("Error running trim command: ", err)
	}

	zeroHash := ZeroHash()
	oneHash := OneHash()
	chunkCount := filesize / TrimChunkSize

	// Write random data to disk
	s.Log("Writing random data to disk: ", trimPath)
	if err := WriteRandomData(trimPath, chunkCount); err != nil {
		s.Fatal("Error writing random data to disk: ", err)
	}

	s.Log("Calculating initial hash values for all chunks")
	initialHash, err := CalculateCurrentHashes(trimPath, chunkCount)
	if err != nil {
		s.Fatal("Error calculating hashes: ", err)
	}

	// Check read bandwidth/latency when reading real data.
	resultWriter := &FioResultWriter{}
	defer resultWriter.Save(ctx, s.OutDir(), true)

	testConfig := &TestConfig{ResultWriter: resultWriter, Path: trimPath}
	if err := runFioStress(ctx, s, testConfig.WithJob("4k_read_qd32")); err != nil {
		s.Fatal("Timeout while running disk i/o stress: ", err)
	}

	dataVerifyCount := 0
	dataVerifyMatch := 0
	trimVerifyCount := 0
	trimVerifyZero := 0
	trimVerifyOne := 0
	trimVerifyNonDelete := 0

	for _, ratio := range []float64{0, 0.25, 0.5, 0.75, 1} {
		trimmed := make([]bool, chunkCount)
		for i := uint64(0); i < chunkCount; i++ {
			if float64(i%4)/4 < ratio {
				if err := RunTrim(f, i, TrimChunkSize); err != nil {
					s.Fatal("Error running trim command: ", err)
				}
				trimmed[i] = true
				trimVerifyCount++
			}
		}

		currHashes, err := CalculateCurrentHashes(trimPath, chunkCount)
		if err != nil {
			s.Fatal("Error calculating current hashes: ", err)
		}

		dataVerifyCount = dataVerifyCount + int(chunkCount) - trimVerifyCount

		for i := uint64(0); i < chunkCount; i++ {
			if trimmed[i] {
				if currHashes[i] == zeroHash {
					trimVerifyZero++
				} else if currHashes[i] == oneHash {
					trimVerifyOne++
				} else if currHashes[i] == initialHash[i] {
					trimVerifyNonDelete++
				}
			} else {
				if currHashes[i] == initialHash[i] {
					dataVerifyMatch++
				}
			}
		}
	}

	// Check final (trimmed) read bandwidth/latency.
	if err := runFioStress(ctx, s, testConfig.WithJob("4k_read_qd32")); err != nil {
		s.Fatal("Timeout while running disk i/o stress")
	}

	// Write out all metrics to an external keyval file.
	if err := WriteKeyVals(s.OutDir(), map[string]float64{
		"dataVerifyCount":     float64(dataVerifyCount),
		"dataVerifyMatch":     float64(dataVerifyMatch),
		"trimVerifyCount":     float64(trimVerifyCount),
		"trimVerifyZero":      float64(trimVerifyZero),
		"trimVerifyOne":       float64(trimVerifyOne),
		"trimVerifyNonDelete": float64(trimVerifyNonDelete),
	}); err != nil {
		s.Fatal("Error writing trim counters: ", err)
	}

	if dataVerifyMatch < dataVerifyCount {
		s.Fatal("Fail to verify untrimmed data")
	}

	if trimVerifyZero < trimVerifyCount {
		s.Fatal("Trimmed data is not zeroed")
	}
}

// runFioStress runs an fio job:
// If fio returns an error, this function will fail the Tast test.
func runFioStress(ctx context.Context, s *testing.State, testConfig TestConfig) error {
	config := testConfig.WithJobFile(s.DataPath(testConfig.Job))
	if err := RunFioStress(ctx, config); err != nil {
		if errors.As(err, &context.DeadlineExceeded) {
			return err
		}
		s.Fatal("FIO stress failed: ", err)
	}
	return nil
}
