// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/storage/stress"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// slcQualConfig is the configuration of dual-qual functionality.
type slcQualConfig struct {
	enabled bool
	device  string
}

// subTestFunc is the code associated with a sub-test.
type subTestFunc func(context.Context, *testing.State, *stress.FioResultWriter, slcQualConfig)

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
		Vars:         []string{"tast_disk_size_gb", "tast_storage_slc_qual"},
		Params: []testing.Param{{
			Name:    "setup_benchmarks",
			Val:     setupBenchmarks,
			Timeout: 1 * time.Hour,
		}, {
			Name:    "stress",
			Val:     stressRunner,
			Timeout: 5 * time.Hour,
		}, {
			Name:    "mini_soak",
			Val:     miniSoakRunner,
			Timeout: 2 * time.Hour,
		}, {
			Name:    "teardown_benchmarks",
			Val:     setupBenchmarks,
			Timeout: 1 * time.Hour,
		}},
	})
}

func swapoff(ctx context.Context) error {
	testing.ContextLog(ctx, "Disabling swap")
	err := testexec.CommandContext(ctx, "swapoff", "-a").Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to turn swap off")
	}

	return nil
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
}

// setupBenchmarks captures and records bandwidth and latency disk benchmarks at the
// beginning and the end of the test suite.
func setupBenchmarks(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
	testConfig := &stress.TestConfig{ResultWriter: rw}

	// Run tests to collect metrics for boot device.
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("seq_write"))
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("seq_read"))
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("4k_write"))
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("4k_read"))
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("16k_write"))
	runFioStress(ctx, s, testConfig.WithPath(stress.BootDeviceFioPath).WithJob("16k_read"))

	if slcConfig.enabled {
		// Run tests to collect metrics for Slc device.
		runFioStress(ctx, s, testConfig.WithPath(slcConfig.device).WithJob("4k_write"))
		runFioStress(ctx, s, testConfig.WithPath(slcConfig.device).WithJob("4k_read"))
	}
}

// soakTestBlock runs long, write-intensive storage stresses.
func soakTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
	testConfigNoVerify := &stress.TestConfig{
		Duration: soakBlockTimeout / 2,
	}
	testConfigVerify := &stress.TestConfig{
		VerifyOnly:   true,
		ResultWriter: rw,
		Duration:     soakBlockTimeout / 2,
	}

	stressTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStress(ctx, s, testConfigNoVerify.WithPath(stress.BootDeviceFioPath).WithJob("64k_stress"))
			runFioStress(ctx, s, testConfigVerify.WithPath(stress.BootDeviceFioPath).WithJob("surfing"))
		},
	}

	if slcConfig.enabled {
		stressTasks = append(stressTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, testConfigNoVerify.WithPath(slcConfig.device).WithJob("4k_write"))
				runFioStress(ctx, s, testConfigVerify.WithPath(slcConfig.device).WithJob("4k_write"))
			})
	}

	runTasksInParallel(ctx, 0, stressTasks)
}

// retentionTestBlock reads and then validates the same data after multiple short suspend cycles.
func retentionTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
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
			runFioStress(ctx, s, writeConfig.WithPath(stress.BootDeviceFioPath))
		},
	}
	verifyTasks := []func(context.Context){
		func(ctx context.Context) {
			runFioStress(ctx, s, verifyConfig.WithPath(stress.BootDeviceFioPath))
		},
	}

	if slcConfig.enabled {
		writeTasks = append(writeTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, writeConfig.WithPath(slcConfig.device))
			})
		verifyTasks = append(verifyTasks,
			func(ctx context.Context) {
				runFioStress(ctx, s, verifyConfig.WithPath(slcConfig.device))
			})
	}

	runTasksInParallel(ctx, 0, writeTasks)

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
	runTasksInParallel(ctx, 0, verifyTasks)
}

// runContinuousStorageStress is a storage stress that is periodically interrupted by a power suspend.
func runContinuousStorageStress(ctx context.Context, job, jobFile string, rw *stress.FioResultWriter, path string) {
	testConfig := stress.TestConfig{
		Path:         path,
		Job:          job,
		JobFile:      jobFile,
		ResultWriter: rw,
	}
	// Running write stress continuously, until timeout.
	for {
		if err := stress.RunFioStress(ctx, testConfig); errors.Is(err, context.DeadlineExceeded) {
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
func suspendTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
	if deadline, _ := ctx.Deadline(); time.Until(deadline) < suspendBlockTimeout {
		s.Fatal("Context timeout occurs before suspend block timeout")
	}

	tasks := []func(context.Context){
		func(ctx context.Context) {
			runContinuousStorageStress(ctx, "write_stress", s.DataPath("write_stress"), rw, stress.BootDeviceFioPath)
		},
		runPeriodicPowerSuspend,
	}

	if slcConfig.enabled {
		tasks = append(tasks,
			func(ctx context.Context) {
				runContinuousStorageStress(ctx, "4k_write", s.DataPath("4k_write"), rw, slcConfig.device)
			})
	}

	runTasksInParallel(ctx, suspendBlockTimeout, tasks)
}

// trimTestBlock is a dispatcher function to start trim test on the boot device
// and on the slc.
func trimTestBlock(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
	bootDevPartition, err := stress.RootPartitionForTrim(ctx)
	if err != nil {
		s.Fatal("Failed to select partition for trim stress: ", err)
	}
	trimTestBlockImpl(ctx, s, bootDevPartition, rw)

	if slcConfig.enabled {
		trimTestBlockImpl(ctx, s, slcConfig.device, rw)
	}
}

// trimTestBlockImpl performs data integrity trim test on an unmounted partition.
// This test will write 1 GB of data and verify that trimmed data are gone and untrimmed data are unaffected.
// The verification will be run in 5 passes with 0%, 25%, 50%, 75%, and 100% of data trimmed.
// Also, perform 4K random read QD32 before and after trim. We should see some speed / latency difference
// if the device firmware trim data properly.
func trimTestBlockImpl(ctx context.Context, s *testing.State, trimPath string, rw *stress.FioResultWriter) {
	filesize, err := stress.PartitionSize(ctx, trimPath)
	if err != nil {
		s.Fatal("Failed to acquire size for partition: ", err)
	}
	// Make file size multiple of 4 * chunk size to account for all passes,
	// i.e. 25% = 1/4, 75% = 3/4.
	filesize = filesize - filesize%(4*stress.TrimChunkSize)
	s.Logf("Filename: %s, filesize: %d", trimPath, filesize)

	f, err := os.OpenFile(trimPath, os.O_RDWR, 0666)
	if err != nil {
		s.Fatal("Failed to open device: ", err)
	}
	defer f.Close()

	if err := stress.RunTrim(f, 0, filesize); err != nil {
		s.Fatal("Error running trim command: ", err)
	}

	zeroHash := stress.ZeroHash()
	oneHash := stress.OneHash()
	chunkCount := filesize / stress.TrimChunkSize

	// Write random data to disk
	s.Log("Writing random data to disk: ", trimPath)
	if err := stress.WriteRandomData(trimPath, chunkCount); err != nil {
		s.Fatal("Error writing random data to disk: ", err)
	}

	s.Log("Calculating initial hash values for all chunks")
	initialHash, err := stress.CalculateCurrentHashes(trimPath, chunkCount)
	if err != nil {
		s.Fatal("Error calculating hashes: ", err)
	}

	// Check read bandwidth/latency when reading real data.
	resultWriter := &stress.FioResultWriter{}
	defer resultWriter.Save(ctx, s.OutDir(), true)

	testConfig := &stress.TestConfig{ResultWriter: resultWriter, Path: trimPath}
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
				if err := stress.RunTrim(f, i, stress.TrimChunkSize); err != nil {
					s.Fatal("Error running trim command: ", err)
				}
				trimmed[i] = true
				trimVerifyCount++
			}
		}

		currHashes, err := stress.CalculateCurrentHashes(trimPath, chunkCount)
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
	if err := stress.WriteKeyVals(s.OutDir(), map[string]float64{
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

// slcDevice returns an Slc device path for dual-namespace AVL.
func slcDevice(ctx context.Context) (string, error) {
	info, err := stress.ReadDiskInfo(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed reading disk info")
	}
	slc, err := info.SlcDevice()
	if slc == nil {
		return "", errors.Wrap(err, "dual qual is specified but SLC device is not present")
	}
	return filepath.Join("/dev/", slc.Name), nil
}

// runTasksInParallel runs stress tasks in parallel until finished or until
// timeout. "0" timeout means no timeout.
// TODO(dlunev, abergman): figure out if we need to have a different way
// to handle premature task cancelation.
func runTasksInParallel(ctx context.Context, timeout time.Duration, tasks []func(ctx context.Context)) {
	if timeout != 0 {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = ctxWithTimeout
	}

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
}

// runFioStress runs an fio job:
// If fio returns an error, this function will fail the Tast test.
func runFioStress(ctx context.Context, s *testing.State, testConfig stress.TestConfig) error {
	config := testConfig.WithJobFile(s.DataPath(testConfig.Job))
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
func stressRunner(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
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
		{
			name:     "trim",
			function: subTestFunc(trimTestBlock),
		},
	} {
		for retries := 0; retries < maxSubtestRetry; retries++ {
			s.Logf("Subtest: %s, retry: %d of %d", tc.name, retries+1, maxSubtestRetry)
			passed := s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
				tc.function(ctx, s, rw, slcConfig)
			})
			if passed {
				break
			}
		}
	}
}

// miniSoakRunner is a minimized version of the storage stress consisting from
// a single attempt of a soak subtest.
// This stress is used for storage qualification v2 validation.
func miniSoakRunner(ctx context.Context, s *testing.State, rw *stress.FioResultWriter, slcConfig slcQualConfig) {
	soakTestBlock(ctx, s, rw, slcConfig)
}

// FullQualificationStress runs a full version of disk IO qualification test.
// The full run of the test can take anything between 2-14 days.
func FullQualificationStress(ctx context.Context, s *testing.State) {
	subtest := s.Param().(func(context.Context, *testing.State, *stress.FioResultWriter, slcQualConfig))
	start := time.Now()

	slcConfig := slcQualConfig{}
	if val, ok := s.Var("tast_storage_slc_qual"); ok {
		var err error
		if slcConfig.enabled, err = strconv.ParseBool(val); err != nil {
			s.Fatal("Cannot parse argumet 'storage.QuickStress.slcQual' of type bool: ", err)
		}
		if slcConfig.enabled {
			// Run tests to collect metrics for Slc device.
			if slcConfig.device, err = slcDevice(ctx); err != nil {
				s.Fatal("Failed to get slc device: ", err)
			}
			if err = swapoff(ctx); err != nil {
				s.Fatal("Failed to turn off the swap: ", err)
			}
		}
	}

	// Before running any functional test block, test setup should be validated.
	passed := s.Run(ctx, "setup_checks", func(ctx context.Context, s *testing.State) {
		setupChecks(ctx, s)
	})
	if passed {
		passed = s.Run(ctx, "storage_subtest", func(ctx context.Context, s *testing.State) {
			resultWriter := &stress.FioResultWriter{}
			defer resultWriter.Save(ctx, s.OutDir(), true)
			subtest(ctx, s, resultWriter, slcConfig)
		})
	}
	if err := stress.WriteTestStatusFile(ctx, s.OutDir(), passed, start); err != nil {
		s.Fatal("Error writing status file: ", err)
	}
}
