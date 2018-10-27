// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

var (
	containerHomeDir = "/home/testuser"
)

func getTestDataFileName(inContainer bool) string {
	const testDataFileName = "fio_test_data"

	var targetFile string
	if inContainer {
		targetFile = filepath.Join(containerHomeDir, testDataFileName)
	} else {
		targetFile = filepath.Join("/mnt", "stateful_partition", testDataFileName)
	}

	return targetFile
}

func fioCmd(ctx context.Context, cont *vm.Container, jobFile string, extraEnvArgs []string) *testexec.Cmd {
	var envArgs []string
	envArgs = append(envArgs, "FILENAME="+getTestDataFileName(cont != nil))
	envArgs = append(envArgs, extraEnvArgs...)

	extraArgs := []string{"--output-format=json"}

	var cmd *testexec.Cmd
	if cont == nil {
		cmd = testexec.CommandContext(ctx, "fio", append([]string{jobFile}, extraArgs...)...)
		cmd.Env = envArgs
	} else {
		cmd = cont.Command(ctx, append(envArgs, append([]string{"fio", jobFile}, extraArgs...)...)...)
	}
	return cmd
}

func runOnce(ctx context.Context, s *testing.State, cmd *testexec.Cmd) (float64, float64, error) {
	out, err := cmd.Output()
	if err != nil {
		s.Log("fio output: ", string(out))
		cmd.DumpLog(ctx)
		return 0, 0, errors.Wrap(err, "failed to run fio")
	}

	var result interface{}
	json.Unmarshal(out, &result)

	jobs := result.(map[string]interface{})["jobs"].([]interface{})
	var totalRead, totalWrite float64 = 0, 0
	for _, job := range jobs {
		read := job.(map[string]interface{})["read"].(map[string]interface{})["bw"].(float64)
		write := job.(map[string]interface{})["write"].(map[string]interface{})["bw"].(float64)
		totalRead += read
		totalWrite += write
	}
	avgRead := totalRead / float64(len(jobs))
	avgWrite := totalWrite / float64(len(jobs))
	s.Logf("avgRead %v, avgWrite %v", avgRead, avgWrite)

	return avgRead, avgWrite, nil
}

func reportMetric(s *testing.State, metricName string, guestValue float64, hostValue float64, perfValues *perf.Values) {
	ratio := guestValue / hostValue
	s.Logf("Reporting metric %v: %v %v %v", metricName, guestValue, hostValue, ratio)
	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "guest_" + metricName,
		Unit:      "KB_per_sec",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, guestValue)

	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "host_" + metricName,
		Unit:      "KB_per_sec",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, hostValue)

	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "ratio_" + metricName,
		Unit:      "percentage",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, ratio)
}

func runOneJobFile(ctx context.Context, s *testing.State, cont *vm.Container, jobFile string, perfValues *perf.Values, report string) error {
	const numTries = 3

	// Delete the test data file on host.
	defer os.Remove(getTestDataFileName(false))

	type fioSetting struct {
		fileSize  string
		blockSize string
		loopNum   int
	}
	settings := []fioSetting{
		{fileSize: "300m", blockSize: "4k", loopNum: 1},
		{fileSize: "600m", blockSize: "64k", loopNum: 1},
		{fileSize: "1G", blockSize: "1m", loopNum: 1},
		{fileSize: "1G", blockSize: "16m", loopNum: 1},
	}

	for _, setting := range settings {
		extraEnvArgs := []string{
			"FILESIZE=" + setting.fileSize,
			"BLOCKSIZE=" + setting.blockSize,
			fmt.Sprintf("LOOPNUM=%v", setting.loopNum),
		}
		baseMetricName := fmt.Sprintf("%v_bs_%v", jobFile, setting.blockSize)

		for i := 1; i <= numTries; i++ {
			s.Logf("Running %v with bs %v in container (%v/%v)", jobFile, setting.blockSize, i, numTries)
			cmd := fioCmd(ctx, cont, filepath.Join(containerHomeDir, jobFile), extraEnvArgs)
			guestRead, guestWrite, err := runOnce(ctx, s, cmd)
			if err != nil {
				return err
			}

			s.Logf("Running %v with bs %v outside container (%v/%v)", jobFile, setting.blockSize, i, numTries)
			cmd = fioCmd(ctx, nil, s.DataPath(jobFile), extraEnvArgs)
			hostRead, hostWrite, err := runOnce(ctx, s, cmd)
			if err != nil {
				return err
			}

			if strings.Contains(report, "w") {
				reportMetric(s, baseMetricName+"_write", guestWrite, hostWrite, perfValues)
			}
			if strings.Contains(report, "r") {
				reportMetric(s, baseMetricName+"_read", guestRead, hostRead, perfValues)
			}
		}
	}
	return nil
}

// DiskIOPerf Run disk IO performance test by running the tool "fio".
func DiskIOPerf(ctx context.Context, s *testing.State, cont *vm.Container, perfValues *perf.Values) error {
	s.Log("Measuring disk IO performance")

	s.Log("Installing fio")
	cmd := cont.Command(ctx, "sh", "-c", "yes | sudo apt-get install fio")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install fio")
	}

	s.Log("Copying job files to container")
	for _, jobFile := range []string{"fio_seq_write", "fio_seq_read", "fio_rand_write", "fio_rand_read", "fio_stress_rw"} {
		containerJobFilePath := filepath.Join(containerHomeDir, jobFile)
		if err := cont.PushFile(ctx, s.DataPath(jobFile), containerJobFilePath); err != nil {
			return errors.Wrapf(err, "failed to push %v to container", jobFile)
		}
	}

	s.Log("Sequential write")
	runOneJobFile(ctx, s, cont, "fio_seq_write", perfValues, "w")

	s.Log("Sequential read")
	runOneJobFile(ctx, s, cont, "fio_seq_read", perfValues, "r")

	s.Log("Random write")
	runOneJobFile(ctx, s, cont, "fio_rand_write", perfValues, "w")

	s.Log("Random read")
	runOneJobFile(ctx, s, cont, "fio_rand_read", perfValues, "r")

	s.Log("Stress rw")
	runOneJobFile(ctx, s, cont, "fio_stress_rw", perfValues, "rw")

	return nil
}
