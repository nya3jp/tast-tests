// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// runContinuousStorageStress is a storage stress that is periodically interrupted by a power suspend.
func runContinuousStorageStress(ctx context.Context, job, jobFile string, rw *FioResultWriter, path string) error {
	testConfig := TestConfig{
		Path:         path,
		Job:          job,
		JobFile:      jobFile,
		ResultWriter: rw,
	}
	// Running write stress continuously, until timeout.
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := RunFioStress(ctx, testConfig); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return nil
}

// runPeriodicPowerSuspend repeatedly suspends the DUT that is running a FIO
// Exits only when context deadline is exceeded.
func runPeriodicPowerSuspend(ctx context.Context, SkipS0iXResidencyCheck bool) error {
	// Indefinite loop of randomized sleeps and power suspends.
	for {
		if ctx.Err() != nil {
			return nil
		}
		sleepDuration := time.Duration(rand.Intn(30)+30) * time.Second
		testing.ContextLog(ctx, "Sleeping for ", sleepDuration)
		testing.Sleep(ctx, sleepDuration)
		if err := Suspend(ctx, SkipS0iXResidencyCheck); err != nil {
			return errors.Wrap(err, "failed to suspend DUT")
		}
	}
}

// runTasksInParallel runs stress tasks in parallel until finished or until
// timeout. "0" timeout means no timeout.
func runTasksInParallel(ctx context.Context, timeout time.Duration, tasks []func(ctx context.Context) error) error {
	var cancel context.CancelFunc = nil
	var firstError error = nil
	var firstErrorLock sync.Mutex

	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	testing.ContextLog(ctx, "Starting parallel tasks at: ", time.Now())

	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(taskToRun func(ctx context.Context) error, taskId int) {
			testing.ContextLog(ctx, "Starting taskId: ", taskId)
			err := taskToRun(ctx)
			testing.ContextLog(ctx, "Finishing taskId: ", taskId)
			firstErrorLock.Lock()
			if err != nil && firstError == nil {
				firstError = errors.Wrap(err, "error while running parallel tasks")
				cancel()
			}
			firstErrorLock.Unlock()
			wg.Done()
		}(task, i)
	}
	wg.Wait()
	testing.ContextLog(ctx, "Finished parallel tasks at: ", time.Now())
	return firstError
}

// StressRunner is the main entry point of the unversal stress block.
// It runs all other functional sub-tests in a sequence, retrying failed sub-tests.
func StressRunner(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	for _, tc := range []struct {
		name     string
		function subTestFunc
		enabled  bool
	}{
		{
			name:     "stressBenchmarks",
			function: subTestFunc(SetupBenchmarks),
			enabled:  true,
		},
		{
			name:     "soak",
			function: subTestFunc(soakTestBlock),
			enabled:  true,
		},
		{
			name:     "suspend",
			function: subTestFunc(suspendTestBlock),
			enabled:  true,
		},
		{
			name:     "retention",
			function: subTestFunc(retentionTestBlock),
			enabled:  !testParam.FollowupQual,
		},
		{
			name:     "trim",
			function: subTestFunc(trimTestBlock),
			enabled:  true,
		},
	} {
		if !tc.enabled {
			s.Logf("Subtest: %s, disabled", tc.name)
			continue
		}
		for retries := 0; retries < maxSubtestRetry; retries++ {
			s.Logf("Subtest: %s, retry: %d of %d", tc.name, retries+1, maxSubtestRetry)
			passed := s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
				tc.function(ctx, s, rw, testParam)
			})
			if passed {
				break
			}
		}
	}
}

// FunctionalRunner exercises only the functional part of the block.
// It is intended to be used in the lab on bringup devices.
func FunctionalRunner(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	for _, tc := range []struct {
		name     string
		function subTestFunc
	}{
		{
			name:     "suspend",
			function: subTestFunc(suspendTestBlock),
		},
		{
			name:     "trim",
			function: subTestFunc(trimTestBlock),
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			tc.function(ctx, s, rw, testParam)
		})
	}
}

// MiniSoakRunner is a minimized version of the storage stress consisting from
// a single attempt of a soak subtest.
// This stress is used for storage qualification v2 validation.
func MiniSoakRunner(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	soakTestBlock(ctx, s, rw, testParam)
}
