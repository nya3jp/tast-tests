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
		Func: TPMContest,
		Desc: "Concurrently tasks the TPM",
		Contacts: []string{
			"markhas@google.com",       // Test author
			"cros-proj-amd@google.com", // Backup mailing list
		},
		SoftwareDeps: []string{"tpm2", "protected_content", "amd64"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
	})
}

// output field is only populated on error
type result struct {
	cmd    string
	err    error
	output []byte
}

// runCmdRepeatedly runs a shell command #iteration times on target
// It will stop execution early if an error occurs or #ctx is cancelled
// If #iterations < 0, the command will be executed infinitely
func runCmdRepeatedly(ctx context.Context, cmd []string, iterations int) result {
	r := libhwseclocal.NewCmdRunner()
	cmdStr := strings.Join(cmd, " ")
	for i := 0; i < iterations || iterations < 0; i++ {
		if output, err := r.RunWithCombinedOutput(ctx, cmd[0], cmd[1:]...); err != nil {
			if err != nil {
				if err == context.Canceled {
					// A cancelled context is a benign error case
					err, output = nil, nil
				}
				return result{cmdStr, err, output}
			}
		}
	}
	return result{cmdStr, nil, nil}
}

// TPMContest stresses the TPM by concurrently tasking it from multiple contexts.
// The test runs until one of the follow occurs:
// - Each shell command successfully runs #minTestIterations times (PASS)
// - Test.Timeout is reached (FAIL)
// - A shell command returns a non-zero status code (FAIL)
func TPMContest(ctx context.Context, s *testing.State) {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	const minTestIterations = 1
	commands := [][]string{{"tpm-manager", "get_random", "4096"},
		{"oemcrypto_hw_ref_tests", "-v", "-v", "-v", "-v",
			"--gtest_filter=*OEMCryptoSessionTests.OEMCryptoMemoryCreateUsageTableHeaderForHugeHeaderBufferLength"},
		{"trunks_client", "--key_create", "--rsa=2048", "--usage=decrypt", "--key_blob=/tmp/key", "--print_time"}}
	done := make(chan result, len(commands)*2)

	for _, cmd := range commands {
		c := cmd
		go func() {
			r := runCmdRepeatedly(workerCtx, c, minTestIterations)
			done <- r
			if r.err == nil {
				r = runCmdRepeatedly(workerCtx, c, -1)
			} else {
				r = result{} // Send default result as cmd was not executed
			}
			done <- r
		}()
	}

	for i := 0; i < len(commands)*2; i++ {
		result := <-done
		if result.err != nil {
			s.Errorf("Error running %s: %v", result.cmd, result.err)
			if result.output != nil {
				s.Log(string(result.output))
			}
		}
		// Cancel workers if:
		// 1. An error occurs
		// 2. All commands successfully run #minTestIterations times
		if cancel != nil && (i == len(commands)-1 || result.err != nil) {
			cancel()
			cancel = nil
		}
	}
}
