// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrostiniNetworkPerf,
		Desc: "Tests Crostini network performance",
		// TODO(cylee): A presubmit check enforce "informational". Confirm if we should remove the checking.
		Attr: []string{"informational", "group:crosbolt", "crosbolt_nightly"},
		// Data:         dataFiles,
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniNetworkPerf(ctx context.Context, s *testing.State) {
	// TODO(cylee): Consolidate container creation logic in a util function since it appears in multiple files.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	// TODO(cylee): Consolidate similar util function in other test files.
	// Prepare error log file.
	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()
	writeError := func(title string, content []byte) {
		const logTemplate = "========== START %s ==========\n%s\n========== END ==========\n"
		if _, err := fmt.Fprintf(errFile, logTemplate, title, content); err != nil {
			s.Log("Failed to write error to log file: ", err)
		}
	}
	runCmd := func(cmd *testexec.Cmd) (out []byte, err error) {
		out, err = cmd.Output()
		if err == nil {
			return out, nil
		}
		cmdString := strings.Join(append(cmd.Cmd.Env, cmd.Cmd.Args...), " ")

		// Dump stderr.
		if err := cmd.DumpLog(ctx); err != nil {
			s.Logf("Failed to dump log for cmd %q: %v", cmdString, err)
		}

		// Output complete stdout to a log file.
		writeError(cmdString, out)

		// Only append the first and last line of the output to the error.
		out = bytes.TrimSpace(out)
		var errSnippet string
		if idx := bytes.IndexAny(out, "\r\n"); idx != -1 {
			lastIdx := bytes.LastIndexAny(out, "\r\n")
			errSnippet = fmt.Sprintf("%s ... %s", out[:idx], out[lastIdx+1:])
		} else {
			errSnippet = string(out)
		}
		return []byte{}, errors.Wrap(err, errSnippet)
	}

	s.Log("Installing iperf3")
	if _, err := runCmd(cont.Command(ctx, "sudo", "apt-get", "-y", "install", "iperf3")); err != nil {
		s.Fatal("Failed to install iperf3: ", err)
	}

	serverCmd := cont.Command(ctx, "iperf3", "-s")
	// Write server logs to a file.
	serverLogFile, err := os.Create(filepath.Join(s.OutDir(), "iperf_serever_log.txt"))
	if err != nil {
		s.Fatal("Failed to create server log file: ", err)
	}
	defer serverLogFile.Close()
	serverCmd.Stdout = serverLogFile
	serverCmd.Stderr = serverLogFile
	// Do not wait for it to finish.
	if err := serverCmd.Start(); err != nil {
		s.Fatal("Failed to run iperf3 server in container: ", err)
	}
	defer func() {
		s.Log("Terminating iperf3 server in container")
		serverCmd.Kill()
	}()

	out, err := runCmd(cont.Command(ctx, "hostname", "-I"))
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	containerIP := strings.TrimSpace(string(out))
	s.Log("Container IP address ", containerIP)

	type iperfSumStruct struct {
		BitsPerSecond float64 `json:"bits_per_second"`
		Seconds       float64 `json:"seconds"`
	}
	type iperfMetrics struct {
		End struct {
			SumSent     iperfSumStruct `json:"sum_sent"`
			SumReceived iperfSumStruct `json:"sum_received"`
		}
	}

	type direction int
	const (
		hostToContainer direction = iota
		containerToHost
	)
	measureBandwidth := func(dir direction) (result iperfMetrics) {
		args := []string{
			"-J",              // JSON output.
			"-c", containerIP, // run iperf3 client instead of server.
		}
		if dir == containerToHost {
			args = append(args, "-R") // reverse direction.
		}
		out, err := runCmd(testexec.CommandContext(ctx, "iperf3", args...))
		if err != nil {
			s.Error("Failed to run iperf3 client command: ", err)
		}
		if err = json.Unmarshal(out, &result); err != nil {
			writeError("parsing iperf3 result", out)
			s.Error("Failed to parse iperf3 output: ", err)
		}
		s.Logf("Finished in %v, bits per seconds %v",
			(time.Duration(result.End.SumSent.Seconds) * time.Second).Round(time.Millisecond),
			result.End.SumSent.BitsPerSecond)
		return result
	}

	perfValues := perf.Values{}

	const repeatNum = 3
	for t := 1; t <= repeatNum; t++ {
		s.Logf("Measuring host to container bandwidth (%d/%d)", t, repeatNum)
		result := measureBandwidth(hostToContainer)
		perfValues.Append(perf.Metric{
			Name:      "crosini_network",
			Variant:   "host_to_container_bandwidth",
			Unit:      "bits_per_sec",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}, result.End.SumSent.BitsPerSecond)

		s.Logf("Measuring container to host bandwidth (%d/%d)", t, repeatNum)
		result = measureBandwidth(containerToHost)
		perfValues.Append(perf.Metric{
			Name:      "crosini_network",
			Variant:   "container_to_host_bandwidth",
			Unit:      "bits_per_sec",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}, result.End.SumReceived.BitsPerSecond)
	}

	perfValues.Save(s.OutDir())
}
