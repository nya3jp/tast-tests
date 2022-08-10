// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"
	"time"

	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TpmContest,
		Desc: "Concurrently tasks the TPM",
		Contacts: []string{
			"markhas@google.com",      // Test author
			"cros-hwsec@chromium.org", // Backup mailing list
		},
		SoftwareDeps: []string{"amd_oemcrypto"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
	})
}

// runCmdUntilFailOrCancel repeatedly runs a shell command on target until
// the commands fails (non zeroexit status) or its context is cancelled.
// #workerID is written to the #iterationComplete channel each time the shell
// command successfully run. #-workerID is written on error.
func runCmdUntilFailOrCancel(ctx context.Context, cmdStr []string, s *testing.State,
	workerID int, iterationComplete chan int) {
	r := libhwseclocal.NewCmdRunner()

	for i := 0; ; i++ {
		if output, err := r.RunWithCombinedOutput(ctx, cmdStr[0], cmdStr[1:]...); err != nil {
			if err != context.Canceled {
				// Any other errors cause a test failure
				s.Errorf("Error running %s: %v", strings.Join(cmdStr, " "), err)
				s.Logf("%s", output)
				iterationComplete <- -workerID
			}
			return
		}
		s.Logf("Successfully ran %s: Iteration %d", strings.Join(cmdStr, " "), i)
		iterationComplete <- workerID
	}
}

// getMin returns the minimum value in a slice. 0 if len(s) == 0
func getMin(s []int) int {
	var min int
	for i, v := range s {
		if i == 0 || v < min {
			min = v
		}
	}
	return min
}

// TpmContest stresses the TPM by concurrently tasking it from multiple contexts.
// The test runs until one of the follow occurs:
// - Each shell command successfully runs #minTestIterations times (PASS)
// - Test.Timeout is reached (FAIL)
// - A shell command returns a non-zero status code (FAIL)
func TpmContest(ctx context.Context, s *testing.State) {
	workerCtx, cancel := context.WithCancel(ctx)
	const minTestIterations = 1
	commands := [][]string{{"tpm-manager", "get_random", "4096"},
		{"oemcrypto_hw_ref_tests", "-v", "-v", "-v", "-v",
			"--gtest_filter=*OEMCryptoSessionTests.OEMCryptoMemoryCreateUsageTableHeaderForHugeHeaderBufferLength"},
		{"trunks_client", "--key_create", "--rsa=2048", "--usage=decrypt", "--key_blob=/tmp/key", "--print_time"}}
	var iterationComplete = make(chan int, len(commands))
	var workerIterations = make([]int, len(commands))

	defer cancel()

	for workerID, cmdStr := range commands {
		go runCmdUntilFailOrCancel(workerCtx, cmdStr, s, workerID, iterationComplete)
	}

	for shouldRun := true; shouldRun && getMin(workerIterations) < minTestIterations; {
		select {
		case <-ctx.Done():
			s.Errorf("Workers failed to finish %d iterations before timeout", minTestIterations)
			shouldRun = false
		case workerID := <-iterationComplete:
			if workerID >= len(workerIterations) {
				// This shouldn't happen
				s.Errorf("Internal test error. Invalid workerID: %u", workerID)
				shouldRun = false
			} else if workerID < 0 {
				s.Errorf("Worker %d encountered an error", -workerID)
				shouldRun = false
			} else {
				workerIterations[workerID]++
			}
		}
	}
}
