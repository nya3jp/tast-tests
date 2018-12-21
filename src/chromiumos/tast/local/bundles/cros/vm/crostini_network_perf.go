// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// getHostIP returns the IPv4 address associated with the TAP virtual network interface on host.
func getHostIP() (ip string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", errors.Wrap(err, "could not get interfaces")
	}
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "vmtap") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", errors.Wrap(err, "could not get interface addresses")
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.IP.String(), nil
			case *net.IPAddr:
				return v.IP.String(), nil
			}
		}
		return "", errors.New("could not find Addr of type IPNet or IPAddr")
	}
	return "", errors.New("could not find vmtap interface")
}

// parsePingMessage parses the output of a ping command.
// It returns a list of round trip times and the packet loss rate.
func parsePingMessage(ctx context.Context, text []byte) (times []float64, lossRate float64) {
	samplePattern := regexp.MustCompile(
		`64 bytes from .*: icmp_seq=\d+ ttl=\d+ time=(\d*\.?\d+) ms`)
	summaryPattern := regexp.MustCompile(
		`(\d+) packets transmitted, (\d+) received, \d+% packet loss, time \d+ms`)

	// Default number if couldn't parse from the output.
	lossRate = math.NaN()
	for _, line := range bytes.Split(bytes.TrimSpace(text), []byte{'\n'}) {
		matched := samplePattern.FindSubmatch(line)
		if len(matched) > 1 {
			time, err := strconv.ParseFloat(string(matched[1]), 64)
			if err != nil {
				// Should be impossible case.
				testing.ContextLogf(ctx, "Failed to parse time %q in ping output", matched[1])
				continue
			}
			times = append(times, time)
			continue
		}

		matched = summaryPattern.FindSubmatch(line)
		if len(matched) > 2 {
			all, err := strconv.Atoi(string(matched[1]))
			if err != nil {
				testing.ContextLogf(ctx, "Failed to parse num packets transmitted %q", matched[1])
				continue
			}
			received, err := strconv.Atoi(string(matched[2]))
			if err != nil {
				testing.ContextLogf(ctx, "Failed to parse num packets received %q", matched[2])
				continue
			}
			if all != 0 {
				lossRate = float64(all-received) / float64(all)
			}
		}
	}
	return
}

// sleepWithContext sleeps for |duration| and honor to context deadline.
func sleepWithContext(ctx context.Context, duration time.Duration) error {
	testing.ContextLog(ctx, "Sleep for ", duration)
	select {
	case <-time.After(duration):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
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
	defer vm.UnmountComponent(ctx)

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

	// Install needed packages.
	for _, pkg := range []string{
		"iperf3",
		"iputils-ping",
	} {
		s.Log("Installing ", pkg)
		if _, err := runCmd(cont.Command(ctx, "sudo", "apt-get", "-y", "install", pkg)); err != nil {
			s.Fatalf("Failed to install %s: %v", pkg, err)
		}
	}

	// Get host and container IP.
	hostIP, err := getHostIP()
	if err != nil {
		s.Fatal("Failed to get host IP address: ", err)
	}
	s.Log("Host IP address ", hostIP)

	out, err := runCmd(cont.Command(ctx, "hostname", "-I"))
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	containerIP := strings.TrimSpace(string(out))
	s.Log("Container IP address ", containerIP)

	// Perf output
	perfValues := perf.Values{}
	defer perfValues.Save(s.OutDir())

	// Measure ping round trip time.
	pingCmd := []string{
		"ping",
		"-c", "15", // number of pings.
		"-W", "3", // timeout of a response in second.
	}

	// Server to container.
	s.Log("Running ping to container")
	out, err = runCmd(testexec.CommandContext(ctx, pingCmd[0], append(pingCmd[1:], containerIP)...))
	if err != nil {
		s.Error("Failed to ping container: ", err)
	}
	RTTs, lossRate := parsePingMessage(ctx, out)
	s.Logf("RTTs to container: %v, loss rate: %.2f", RTTs, lossRate)
	perfValues.Append(perf.Metric{
		Name:      "crosini_network",
		Variant:   "host_to_container_ping_RTTs",
		Unit:      "milliseoncds",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, RTTs...)
	perfValues.Set(perf.Metric{
		Name:      "crosini_network",
		Variant:   "host_to_container_ping_loss_rate",
		Unit:      "percentage",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, lossRate)

	// Container to host.
	s.Log("Running ping to host")
	out, err = runCmd(cont.Command(ctx, append(pingCmd, hostIP)...))
	if err != nil {
		s.Error("Failed to ping host: ", err)
	}
	RTTs, lossRate = parsePingMessage(ctx, out)
	s.Logf("RTTs to host: %v, loss rate: %.2f", RTTs, lossRate)
	perfValues.Append(perf.Metric{
		Name:      "crosini_network",
		Variant:   "container_to_host_ping_RTTs",
		Unit:      "milliseoncds",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, RTTs...)
	perfValues.Set(perf.Metric{
		Name:      "crosini_network",
		Variant:   "container_to_host_ping_loss_rate",
		Unit:      "percentage",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, lossRate)

	// Measure bandwidth.
	s.Log("Starting iperf3 server")
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

	const repeatNum = 3
	// Pause a bit before iperf3 client connects to the server. Without this a "Connection refused" error
	// occurs from time to time. Perhaps the server needs some time to cleanup the previous connection to
	// returns to ready state.
	const pauseDuration = 3 * time.Second
	for t := 1; t <= repeatNum; t++ {
		if err := sleepWithContext(ctx, pauseDuration); err != nil {
			s.Errorf("Failed to pause for %v: %v", pauseDuration, err)
		}
		s.Logf("Measuring host to container bandwidth (%d/%d)", t, repeatNum)
		result := measureBandwidth(hostToContainer)
		perfValues.Append(perf.Metric{
			Name:      "crosini_network",
			Variant:   "host_to_container_bandwidth",
			Unit:      "bits_per_sec",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}, result.End.SumSent.BitsPerSecond)

		if err := sleepWithContext(ctx, pauseDuration); err != nil {
			s.Errorf("Failed to pause for %v: %v", pauseDuration, err)
		}
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
}
