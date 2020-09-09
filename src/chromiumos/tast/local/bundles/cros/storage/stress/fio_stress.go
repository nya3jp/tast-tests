// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/storage"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	defaultFileSizeBytes = 1024 * 1024 * 1024
)

var (
	// Configs lists all supported fio configurations.
	Configs = []string{
		"surfing",
		"seq_write",
		"seq_read",
		"4k_write",
		"4k_write_qd32",
		"4k_read",
		"4k_read_qd32",
		"16k_write",
		"16k_read",
		"64k_stress",
		"8k_async_randwrite",
		"8k_read",
		"1m_stress",
		"1m_write",
	}
)

// TestConfig provides extra test configuration arguments.
type TestConfig struct {
	// Duration is a minimal duration that the stress should be running for.
	// If single run of the stress takes less than this time, it's going
	// to be repeated until the total running time is greater than this duration.
	Duration time.Duration

	// VerifyOnly if true, make benchmark data is collected to result-chart.json
	// without running the actual stress.
	VerifyOnly bool

	// ResultWriter references the result processing object.
	ResultWriter *FioResultWriter
}

// runFIO runs "fio" storage stress with a given config (jobFile), parses the output JSON and returns the result.
// If verifyOnly is true, represents a benchmark collection run.
func runFIO(ctx context.Context, testDataPath, jobFile string, verifyOnly bool) (*FioResult, error) {
	verifyFlag := "0"
	if verifyOnly {
		verifyFlag = "1"
	}

	cmd := testexec.CommandContext(ctx, "fio", jobFile, "--output-format=json", "--end_fsync=1")
	cmd.Env = []string{
		"FILENAME=" + testDataPath,
		"FILESIZE=" + strconv.Itoa(defaultFileSizeBytes),
		"VERIFY_ONLY=" + verifyFlag,
		"CONTINUE_ERRORS=verify",
	}
	testing.ContextLog(ctx, "Environment: ", cmd.Env)
	testing.ContextLog(ctx, "Running command: ", cmd)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run fio")
	}
	result := &FioResult{}
	if err := json.Unmarshal(out, result); err != nil {
		return nil, errors.Wrap(err, "failed to parse fio result")
	}
	return result, nil
}

func getStorageInfo(ctx context.Context) (*storage.Info, error) {
	info, err := storage.Get(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get storage info")
	}
	if info.Status == storage.Failing {
		return nil, errors.New("storage device is failing, consider removing from DUT farm")
	}
	testing.ContextLogf(ctx, "Storage name: %s, info: %v, type: %v", info.Name, info.Device, info.Status)
	return info, nil
}

func validateTestConfig(ctx context.Context, s *testing.State, job string) {
	for _, config := range Configs {
		if job == config {
			return
		}
	}
	s.Fatalf("job = %q, want one of %q", job, Configs)
}

// escapeJSONName replaces all invalid characters that might be present in the device name to appear
// correctly in the perf result JSON.
func escapeJSONName(name string) string {
	return regexp.MustCompile(`[[\]<>{} ]`).ReplaceAllString(name, "_")
}

func resultGroupName(ctx context.Context, res *FioResult, dev string, rawDevTest bool) string {
	devName := res.DiskUtil[0].Name
	devSize, err := diskSizePretty(devName)
	if err != nil {
		testing.ContextLog(ctx, "Error acquiring size of device: ", devName, err)
		devSize = ""
	}
	testing.ContextLogf(ctx, "Size of device: %s is: %s", devName, devSize)
	group := dev + "_" + string(devSize)
	if rawDevTest {
		group += "_raw"
	}

	return group
}

// RunFioStress runs a single given storage job. job must be in Configs.
// If verifyOnly is true, only benchmark data is collected to result-chart.json
func RunFioStress(ctx context.Context, s *testing.State, job, testDataPath string, testConfig *TestConfig) {
	if testConfig == nil {
		testConfig = &TestConfig{}
	}

	rawDev := strings.HasPrefix(testDataPath, "/dev/")

	validateTestConfig(ctx, s, job)

	// Get device status/info.
	info, err := getStorageInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get storage info: ", err)
	}
	devName := escapeJSONName(info.Name)

	if !rawDev {
		// Delete the test data file on host.
		defer os.RemoveAll(testDataPath)
	}

	testing.ContextLog(ctx, "Running job ", job)
	var res *FioResult
	for start := time.Now(); ; {
		res, err = runFIO(ctx, testDataPath, s.DataPath(job), testConfig.VerifyOnly)
		if err != nil {
			s.Errorf("%v failed: %v", job, err)
		}

		// If duration test parameter is 0, we do a single iteration of a test.
		if testConfig.Duration == 0 || time.Since(start) > testConfig.Duration {
			break
		}
	}

	if testConfig.ResultWriter != nil {
		group := resultGroupName(ctx, res, devName, rawDev)
		testConfig.ResultWriter.Report(group, res)
	}
}

// RunFioStressForBootDevice runs a single given storage job for boot device.
// job must be in Configs.
// If verifyOnly is true, only benchmark data is collected to result-chart.json
func RunFioStressForBootDevice(ctx context.Context, s *testing.State, job string, testConfig *TestConfig) {
	RunFioStress(ctx, s, job, "/mnt/stateful_partition/fio_test_data", testConfig)
}
