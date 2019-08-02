// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		Func:     CrostiniNetworkPerf,
		Desc:     "Tests Crostini network performance",
		Contacts: []string{"cylee@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// Data:         dataFiles,
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// getVmtapIP returns the IPv4 address associated with the TAP virtual network interface on host.
func getVmtapIP() (ip string, err error) {
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
			return "", errors.Wrapf(err, "could not get addresses of interface %q", iface.Name)
		}
		for _, addr := range addrs {
			if v, ok := addr.(*net.IPNet); ok && v.IP.To4() != nil {
				return v.IP.String(), nil
			}
		}
		return "", errors.Errorf("could not find IPv4 address in %q", iface.Name)
	}
	return "", errors.New("could not find vmtap interface")
}

// parsePingMessage parses the output of a ping command.
// It returns a list of round trip times and the packet loss rate in the range [0,1].
// If any error occurs, returns a nil slice for RTTs and 1.0 for loss rate alone with the error itself.
func parsePingMessage(text string) (rtts []time.Duration, lossRate float64, err error) {
	samplePattern := regexp.MustCompile(
		`64 bytes from .*: icmp_seq=\d+ ttl=\d+ time=(\d*\.?\d+) ms`)
	summaryPattern := regexp.MustCompile(
		`(\d+) packets transmitted, (\d+) received, \d+% packet loss, time \d+ms`)

	summaryLineFound := false
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		matched := samplePattern.FindStringSubmatch(line)
		if matched != nil {
			rtt, err := strconv.ParseFloat(matched[1], 64)
			if err != nil {
				return nil, 1.0, errors.Wrapf(err, "failed to parse time %q in ping output", matched[1])
			}
			// Duration is an alias of int64, so in case rtt can be < 1 one needs to convert it to a float first.
			rtts = append(rtts, time.Duration(float64(rtt)*float64(time.Millisecond)))
			continue
		}

		matched = summaryPattern.FindStringSubmatch(line)
		if matched != nil {
			summaryLineFound = true
			all, err := strconv.Atoi(matched[1])
			if err != nil {
				return nil, 1.0, errors.Wrapf(err, "failed to parse num packets transmitted %q", matched[1])
			}
			received, err := strconv.Atoi(matched[2])
			if err != nil {
				return nil, 1.0, errors.Wrapf(err, "failed to parse num packets received %q", matched[2])
			}
			if all != 0 {
				lossRate = float64(all-received) / float64(all)
			} else {
				return nil, 1.0, errors.New("num packets received = 0")
			}
		}
	}
	if !summaryLineFound {
		return nil, 1.0, errors.New("failed to parse loss rate from ping output message")
	}
	return rtts, lossRate, nil
}

// toMilliseconds turns time.Duration to milliseconds in type float64
func toMilliseconds(ts ...time.Duration) (ms []float64) {
	for _, t := range ts {
		ms = append(ms, float64(t)/float64(time.Millisecond))
	}
	return ms
}

// A type to facilitate bidirectional network test.
type connDirection int

const (
	hostToContainer connDirection = iota
	containerToHost
)

func (dir connDirection) metricName(name string) string {
	var prefix string
	if dir == hostToContainer {
		prefix = "host_to_container"
	} else {
		prefix = "container_to_host"
	}
	return fmt.Sprintf("%s_%s", prefix, name)
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
	cont, err := vm.CreateDefaultVMContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer, "", false)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)
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

	// TODO(cylee): remove the installation code once CL:1389056 is submitted.
	// Install needed packages.
	packages := []string{
		"iperf3",
		"iputils-ping",
	}
	s.Log("Installing ", packages)
	installCmdArgs := append([]string{"sudo", "apt-get", "-y", "install"}, packages...)
	if _, err := runCmd(cont.Command(ctx, installCmdArgs...)); err != nil {
		s.Fatalf("Failed to install needed packages %v: %v", packages, err)
	}

	// Get host and container IP.
	hostIP, err := getVmtapIP()
	if err != nil {
		s.Fatal("Failed to get host IP address: ", err)
	}
	s.Log("Host IP address ", hostIP)

	containerIP, err := cont.GetIPv4Address(ctx)
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	s.Log("Container IP address ", containerIP)

	// Perf output
	perfValues := perf.NewValues()
	defer perfValues.Save(s.OutDir())

	// Measure ping round trip time.
	var measurePing = func(dir connDirection) error {
		pingArgs := []string{
			"ping",
			"-c", "15", // number of pings.
			"-W", "3", // timeout of a response in second.
		}
		var pingCmd *testexec.Cmd
		if dir == hostToContainer {
			pingCmd = testexec.CommandContext(ctx, pingArgs[0], append(pingArgs[1:], containerIP)...)
		} else {
			pingCmd = cont.Command(ctx, append(pingArgs, hostIP)...)
		}
		out, err := runCmd(pingCmd)
		if err != nil {
			return errors.Wrap(err, "failed to run ping command")
		}
		rtts, lossRate, err := parsePingMessage(string(out))
		if err != nil {
			writeError("parsing ping result", out)
			return errors.Wrap(err, "failed to parse ping message")
		}
		s.Logf("Ping reported RTTs %v with %.2f loss rate", rtts, lossRate)
		perfValues.Append(
			perf.Metric{
				Name:      "crostini_network",
				Variant:   dir.metricName("ping_rtts"),
				Unit:      "milliseoncds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			},
			toMilliseconds(rtts...)...)
		perfValues.Set(
			perf.Metric{
				Name:      "crostini_network",
				Variant:   dir.metricName("ping_loss_rate"),
				Unit:      "percentage",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			lossRate)
		return nil
	}
	// Server to container.
	s.Log("Running ping to container")
	err = measurePing(hostToContainer)
	if err != nil {
		s.Error("Failed to ping container: ", err)
	}
	// Container to host.
	s.Log("Running ping to host")
	err = measurePing(containerToHost)
	if err != nil {
		s.Error("Failed to ping host: ", err)
	}

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
		Error string `json:"error"`
	}
	// A util function generator for collaborating with testing.Poll(). See usage below.
	measureBandwidthFunc := func(dir connDirection) func(context.Context) error {
		return func(ctx context.Context) error {
			args := []string{
				"-J",              // JSON output.
				"-c", containerIP, // run iperf3 client instead of server.
			}
			if dir == containerToHost {
				args = append(args, "-R") // reverse direction.
			}
			out, err := runCmd(testexec.CommandContext(ctx, "iperf3", args...))
			if err != nil {
				return errors.Wrap(err, "failed to run iperf3 client command")
			}
			var result iperfMetrics
			if err = json.Unmarshal(out, &result); err != nil {
				writeError("parsing iperf3 result", out)
				return errors.Wrap(err, "failed to parse iperf3 output")
			}
			if result.Error != "" {
				return errors.Errorf("iperf3 returns error %q", result.Error)
			}
			var summary iperfSumStruct
			if dir == hostToContainer {
				summary = result.End.SumSent
			} else {
				summary = result.End.SumReceived
			}
			s.Logf("Finished in %v, bits per seconds %v",
				time.Duration(summary.Seconds*float64(time.Second)).Round(time.Millisecond),
				summary.BitsPerSecond)
			perfValues.Append(perf.Metric{
				Name:      "crostini_network",
				Variant:   dir.metricName("iperf_bandwidth"),
				Unit:      "bits_per_sec",
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			}, summary.BitsPerSecond)
			return nil
		}
	}

	const repeatNum = 3
	// Sometimes iperf returns a "Connection refused" error for the first run or between consecutive runs.
	// Perhaps iperf3 server needs some time to setup or cleanup the previous connection to  get into
	// ready state. So using a poll here.
	iperfPollOption := &testing.PollOptions{
		Timeout:  time.Minute,
		Interval: time.Second,
	}
	for t := 1; t <= repeatNum; t++ {
		s.Logf("Measuring host to container bandwidth (%d/%d)", t, repeatNum)
		err := testing.Poll(ctx, measureBandwidthFunc(hostToContainer), iperfPollOption)
		if err != nil {
			s.Error("Error measuring host to container bandwidth: ", err)
		}

		s.Logf("Measuring container to host bandwidth (%d/%d)", t, repeatNum)
		err = testing.Poll(ctx, measureBandwidthFunc(containerToHost), iperfPollOption)
		if err != nil {
			s.Error("Error measuring container to host bandwidth: ", err)
		}
	}
}
