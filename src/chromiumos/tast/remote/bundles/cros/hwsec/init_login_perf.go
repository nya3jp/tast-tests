// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: InitLoginPerf,
		Desc: "OOBE and regular login performance test",
		Attr: []string{
			"hwsec_destructive_crosbolt_weekly",
			"group:hwsec_destructive_crosbolt",
		},
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		ServiceDeps: []string{
			"tast.cros.example.ChromeService",
			"tast.cros.hwsec.AttestationDBusService",
		},
		SoftwareDeps: []string{"reboot", "tpm", "chrome"},
		Timeout:      15 * time.Minute,
	})
}

type initLoginBenchmark struct {
	name    string
	display string
}

const numIterations = 7

var defaultPollOptions = &testing.PollOptions{Timeout: 30 * time.Second}
var benchmarks = []initLoginBenchmark{
	{name: "initial_login", display: "1stLogin"},
	{name: "regular_login", display: "RegLogin"},
	{name: "prepare_attestation", display: "PrepAttn"},
}

// saveBootstatSummary saves the bootstat summary log for dubugging purposes
func saveBootstatSummary(ctx context.Context, cmd *hwsecremote.CmdRunnerRemote, outDir string, number int) error {
	var data []byte
	var err error
	if data, err = cmd.Run(ctx, "bootstat_summary"); err != nil {
		return errors.Wrap(err, "failed to run bootstat_summary")
	}
	logPath := filepath.Join(outDir, "bootstat_summary"+"."+strconv.Itoa(number))
	if err := ioutil.WriteFile(logPath, data, 0644); err != nil {
		return errors.Wrapf(err, "failed to write to log %q", logPath)
	}
	return nil
}

// uptimeFromTimestamp returns the uptime duration from the bootstat_summary timestamp.
func uptimeFromTimestamp(ctx context.Context, cmd *hwsecremote.CmdRunnerRemote, name string) (time.Duration, error) {
	var timestamp int
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var data []byte
		if data, err = cmd.Run(ctx, "bootstat_summary", name); err != nil {
			return errors.Wrap(err, "failed to run bootstat_summary")
		}
		trimmed := strings.Trim(string(data), "\n")
		records := strings.Split(trimmed, "\n")
		if len(records) < 2 {
			return errors.Errorf("no record of %q in bootstat_summary", name)
		}
		latestRecord := records[len(records)-1]
		testing.ContextLog(ctx, "record: ", latestRecord)
		fields := strings.Fields(latestRecord)
		if len(fields) < 1 {
			return errors.Errorf("no entry in latest record of %q", name)
		}
		if timestamp, err = strconv.Atoi(fields[0]); err != nil {
			return errors.Wrap(err, "failed to convert timestamp to integer")
		}
		return nil
	}, defaultPollOptions); err != nil {
		return -1, errors.Wrapf(err, "timeout waiting for timestamp of %q", name)
	}
	if timestamp < 0 {
		return -1, errors.New("invalid timestamp")
	}
	return time.Duration(timestamp) * time.Millisecond, nil
}

// getLoginDuration returns the login duration by measuring the difference between the timestamps of login-prompt-visible and login-success
func getLoginDuration(ctx context.Context, s *testing.State, cmd *hwsecremote.CmdRunnerRemote) (time.Duration, error) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		return -1, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	cr := example.NewChromeServiceClient(cl.Conn)
	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		return -1, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx, &empty.Empty{})

	var visibleTime, successTime time.Duration
	if visibleTime, err = uptimeFromTimestamp(ctx, cmd, "login-prompt-visible"); err != nil {
		return -1, errors.Wrap(err, "failed to get uptime from timestamp")
	}
	if successTime, err = uptimeFromTimestamp(ctx, cmd, "login-success"); err != nil {
		return -1, errors.Wrap(err, "failed to get uptime from timestamp")
	}
	diffTime := successTime - visibleTime
	return diffTime, nil
}

func waitForAttestationPrepared(ctx context.Context, attestation *hwsec.AttestationClient) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if result, err := attestation.IsPreparedForEnrollment(ctx); err != nil {
			return errors.Wrap(err, "failed to check if attestation is prepared for enrollment")
		} else if !result {
			return errors.New("attestation is not prepared for enrollment")
		}
		return nil
	}, defaultPollOptions); err != nil {
		return err
	}
	return nil
}

// getAttestationInitDuration returns the duration of attestation to get prepared
func getAttestationInitDuration(ctx context.Context, cmd *hwsecremote.CmdRunnerRemote) (time.Duration, error) {
	const logPath = "/var/log/messages"
	const preparedLog = "Attestation: Prepared successfully"
	var line string
	if result, err := cmd.Run(ctx, "grep", preparedLog, logPath); err != nil {
		return -1, errors.Wrap(err, "failed to grep the syslog")
	} else if len(result) == 0 {
		return -1, errors.New("failed to find any message about whether attestation is prepared")
	} else {
		trimmed := strings.Trim(string(result), "\n")
		lines := strings.Split(trimmed, "\n")
		line = lines[len(lines)-1]
	}

	re := regexp.MustCompile(`Prepared successfully \((\d+)ms\)`)
	result := re.FindStringSubmatch(line)
	if len(result) < 2 {
		return -1, errors.Errorf("Init duration does not exist in the message %q", line)
	}
	rawDuration, err := strconv.Atoi(result[1])
	if err != nil {
		return -1, errors.Wrap(err, "failed to convert result to integer")
	}
	duration := time.Duration(rawDuration) * time.Millisecond
	return duration, nil
}

func displayPerfLine(ctx context.Context, res []float64) {
	out := "#"
	for _, v := range res {
		out += fmt.Sprintf(" %8.2f", v)
	}
	testing.ContextLog(ctx, out)
}

func displayPerfStat(ctx context.Context, results map[string]([]float64), name string, f func([]float64) float64) {
	testing.ContextLog(ctx, "# ", name, ":")
	var res []float64
	for _, b := range benchmarks {
		res = append(res, f(results[b.name]))
	}
	displayPerfLine(ctx, res)
}

func displayPerfData(ctx context.Context, results map[string]([]float64)) {
	mean := func(x []float64) float64 {
		sum := 0.0
		for _, v := range x {
			sum += v
		}
		return sum / float64(len(x))
	}
	min := func(x []float64) float64 {
		res := x[0]
		for _, v := range x[1:] {
			res = math.Min(res, v)
		}
		return res
	}
	max := func(x []float64) float64 {
		res := x[0]
		for _, v := range x[1:] {
			res = math.Max(res, v)
		}
		return res
	}
	stdDev := func(x []float64) float64 {
		avg := mean(x)
		res := 0.0
		for _, v := range x {
			res += math.Pow(v-avg, 2)
		}
		return math.Sqrt(res / float64(len(x)-1))
	}
	header := "#"
	for _, b := range benchmarks {
		header += " " + b.display
	}
	testing.ContextLog(ctx, "Benchmark results:")
	testing.ContextLog(ctx, "##############################################")
	testing.ContextLog(ctx, header)
	for iter := 0; iter < numIterations; iter++ {
		var res []float64
		for _, b := range benchmarks {
			res = append(res, results[b.name][iter])
		}
		displayPerfLine(ctx, res)
	}
	if numIterations > 1 {
		displayPerfStat(ctx, results, "Average", mean)
		displayPerfStat(ctx, results, "Min", min)
		displayPerfStat(ctx, results, "Max", max)
		displayPerfStat(ctx, results, "StdDev", stdDev)
	}
	testing.ContextLog(ctx, "##############################################")
}

func InitLoginPerf(ctx context.Context, s *testing.State) {
	cmd := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewFullHelper(cmd, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create helper: ", err)
	}
	tpmManager := helper.TPMManagerClient()
	attestation := helper.AttestationClient()

	results := map[string]([]float64){}
	for iter := 0; iter < numIterations; iter++ {
		s.Log("Start iteration ", iter)
		// Measure OOBE login and init duration.
		if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
			s.Error("Failed to ensure resetting TPM: ", err)
			continue
		}
		if _, err := tpmManager.TakeOwnership(ctx); err != nil {
			s.Error("Failed to take ownership: ", err)
			continue
		}
		firstRes, err := getLoginDuration(ctx, s, cmd)
		if err != nil {
			s.Error("Failed to measure OOBE login duration: ", err)
			continue
		}
		if err := saveBootstatSummary(ctx, cmd, s.OutDir(), iter); err != nil {
			s.Error("Failed to save bootstat summary: ", err)
			continue
		}

		if err := waitForAttestationPrepared(ctx, attestation); err != nil {
			s.Error("Timeout waiting for attestation to be prepared for enrollment: ", err)
			continue
		}
		prepRes, err := getAttestationInitDuration(ctx, cmd)
		if err != nil {
			s.Error("Failed to get attestation init-duration: ", err)
			continue
		}

		// Measure regular login duration
		if err := helper.Reboot(ctx); err != nil {
			s.Error("Failed to reboot: ", err)
			continue
		}
		regRes, err := getLoginDuration(ctx, s, cmd)
		if err != nil {
			s.Error("Failed to measure regular login duration: ", err)
			continue
		}

		results["initial_login"] = append(results["initial_login"], firstRes.Seconds())
		results["prepare_attestation"] = append(results["prepare_attestation"], prepRes.Seconds())
		results["regular_login"] = append(results["regular_login"], regRes.Seconds())
	}
	// Upload the perf measurements.
	value := perf.NewValues()
	for _, b := range benchmarks {
		value.Append(perf.Metric{
			Name:      b.name,
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, results[b.name]...)
	}
	value.Save(s.OutDir())
	// Log the perf results
	displayPerfData(ctx, results)
}
