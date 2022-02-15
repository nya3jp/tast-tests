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
func runContinuousStorageStress(ctx context.Context, job, jobFile string, rw *FioResultWriter, path string) {
	testConfig := TestConfig{
		Path:         path,
		Job:          job,
		JobFile:      jobFile,
		ResultWriter: rw,
	}
	// Running write stress continuously, until timeout.
	for {
		if err := RunFioStress(ctx, testConfig); errors.Is(err, context.DeadlineExceeded) {
			return // Timeout exceeded.
		}
	}
}

// runPeriodicPowerSuspend repeatedly suspends the DUT that is running a FIO
// Exits only when context deadline is exceeded.
func runPeriodicPowerSuspend(ctx context.Context, SkipS0iXResidencyCheck bool) {
	// Indefinite loop of randomized sleeps and power suspends.
	for {
		sleepDuration := time.Duration(rand.Intn(30)+30) * time.Second
		testing.ContextLog(ctx, "Sleeping for ", sleepDuration)
		if err := testing.Sleep(ctx, sleepDuration); errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err := Suspend(ctx, SkipS0iXResidencyCheck); err != nil {
			if errors.As(err, &context.DeadlineExceeded) {
				return
			}
			testing.ContextLog(ctx, "Error suspending system: ", err)
		}
	}
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
	for i, task := range tasks {
		wg.Add(1)
		go func(taskToRun func(ctx context.Context), taskId int) {
			testing.ContextLog(ctx, "Starting taskId: ", taskId)
			taskToRun(ctx)
			testing.ContextLog(ctx, "Finishing taskId: ", taskId)
			wg.Done()
		}(task, i)
	}
	wg.Wait()
	testing.ContextLog(ctx, "Finished parallel tasks at: ", time.Now())
}

// StressRunner is the main entry point of the unversal stress block.
// It runs all other functional sub-tests in a sequence, retrying failed sub-tests.
func StressRunner(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	for _, tc := range []struct {
		name     string
		function subTestFunc
	}{
		{
			name:     "stressBenchmarks",
			function: subTestFunc(SetupBenchmarks),
		},
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

// RemovableRunner exercises the functionality of the block.
// It is intended to be used for removable devices.
func RemovableRunner(ctx context.Context, s *testing.State, rw *FioResultWriter, testParam QualParam) {
	for _, tc := range []struct {
		name     string
		function subTestFunc
	}{
		{
			name:     "stressBenchmarks",
			function: subTestFunc(SetupBenchmarks),
		},
		{
			name:     "suspend",
			function: subTestFunc(suspendTestBlock),
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			tc.function(ctx, s, rw, testParam)
		})
	}
}
