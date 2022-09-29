// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TPMContest,
		Desc: "Concurrently tasks the TPM",
		Contacts: []string{
			"markhas@google.com",       // Test author
			"cros-proj-amd@google.com", // Backup mailing list
		},
		SoftwareDeps: []string{"tpm2", "protected_content", "amd_cpu"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
	})
}

// runCmdRepeatedly runs a shell command #iteration times on target
// It will stop execution early if an error occurs or #ctx is cancelled
// If #iterations < 0, the command will be executed infinitely
// Returns (cmd, cmdOutput, error), cmdOutput is only populated on error
func runCmdRepeatedly(ctx context.Context, cmd []string, iterations int) (string, []byte, error) {
	r := libhwseclocal.NewLoglessCmdRunner()
	cmdStr := strings.Join(cmd, " ")
	for i := 0; i < iterations || iterations < 0; i++ {
		if output, err := r.RunWithCombinedOutput(ctx, cmd[0], cmd[1:]...); err != nil {
			if err != nil {
				if err == context.Canceled {
					// A cancelled context is a benign error case
					err, output = nil, nil
				}
				return cmdStr, output, err
			}
		}
	}
	return cmdStr, nil, nil
}

// TPMContest stresses the TPM by concurrently tasking it from multiple contexts.
// The test runs until one of the follow occurs:
// - Each shell command successfully runs #minTestIterations times (PASS)
// - Test.Timeout is reached (FAIL)
// - A shell command returns a non-zero status code (FAIL)
//
// This test uses go routines to execute the commands in parallel.
// Execution results from each routine are written to a channel where the parent
// thread consumes them. Each go routine will continue to execute its respective
// command until one of the above conditions occurs.
func TPMContest(ctx context.Context, s *testing.State) {

	// output field is only populated on error
	type result struct {
		cmd    string
		output []byte
		err    error
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	const minTestIterations = 50
	commands := [][]string{{"cat", "/sys/class/tpm/tpm0/ph_enable"},
		// oemcrypto_hw_ref_tests tasks the TPM from a trusted application context
		{"oemcrypto_hw_ref_tests", "-v", "-v", "-v", "-v",
			"--gtest_filter=*OEMCryptoSessionTests.OEMCryptoMemoryCreateUsageTableHeaderForHugeHeaderBufferLength"}}

	// A result is written to the #done channel every time #runCmdRepeatedly finishes executing.
	// For this test, every worker thread calls #runCmdRepeatedly twice. The first call runs a
	// command #minTestIterations times. The seconds call runs the same command infinitley until
	// all other commands finish executing a minimum of #minTestIterations times.
	// In the worst case, len(commands)*2 results could be written to the channel
	// before they are consumed. This could happen if all workers finish executing before the
	// caller has a chance to read any of the results.
	done := make(chan result, len(commands)*2)

	for _, cmd := range commands {
		c := cmd
		go func() {
			var r result
			s.Logf("Running %s a minimum of %d times", c, minTestIterations)
			r.cmd, r.output, r.err = runCmdRepeatedly(workerCtx, c, minTestIterations)
			done <- r
			if r.err == nil {
				// Continue to stress the TPM until all commands have finished
				// exeucting #minTestIterations
				r.cmd, r.output, r.err = runCmdRepeatedly(workerCtx, c, -1)
			} else {
				// Send default result as an error already occurred
				// the command will not be executed infinitely
				r.output, r.err = nil, nil
			}
			done <- r
		}()
	}

	// Wait for the worker threads to report execution results. Each worker is expected to write
	// to the channel twice: once after running a command #minTestIterations times, and once after
	// breaking out of its infinite execution loop (either due to a cancelled context, or an error).
	for i := 0; i < len(commands)*2; i++ {
		result := <-done
		if result.err != nil {
			s.Errorf("Error running %s: %v", result.cmd, result.err)
			if result.output != nil {
				path := filepath.Join(s.OutDir(), "failed_cmd_output.txt")
				if err := ioutil.WriteFile(path, result.output, 0644); err != nil {
					s.Errorf("Failed to write command output to %v: %v", path, err)
				} else {
					s.Log("Path on DUT to command output: ", path)
				}
			}
		}
		// Cancel workers if:
		// 1. An error occurs
		// 2. All commands successfully run #minTestIterations times
		if cancel != nil && (result.err != nil || i == len(commands)-1) {
			cancel()
			cancel = nil
		}
	}
}
