// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/storage"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	defaultFileSizeBytes = 1024 * 1024 * 1024
	// BootDeviceFioPath is the pass to test the boot device with.
	BootDeviceFioPath = "/mnt/stateful_partition/fio_test_data"
)

var (
	// Configs lists all supported fio configurations.
	Configs = []string{
		"surfing",
		"recovery",
		"seq_write",
		"seq_read",
		"4k_write",
		"4k_write_qd4",
		"4k_write_qd32",
		"4k_read",
		"4k_read_qd4",
		"4k_read_qd32",
		"16k_write",
		"16k_read",
		"64k_stress",
		"8k_async_randwrite",
		"8k_read",
		"1m_stress",
		"1m_write",
		"write_stress",
	}
)

// TestConfig provides extra test configuration arguments.
type TestConfig struct {
	// Duration is a minimal duration that the stress should be running for.
	// If single run of the stress takes less than this time, it's going
	// to be repeated until the total running time is greater than this duration.
	Duration time.Duration

	// Job is the name of the fio profile to execute. Must be on of the Configs.
	Job string

	// JobFile is the absolute path and filename of the fio profile file corresponding to Job.
	JobFile string

	// Path to the fio target
	Path string

	// VerifyOnly if true, make benchmark data is collected to result-chart.json
	// without running the actual stress.
	VerifyOnly bool

	// ResultWriter references the result processing object.
	ResultWriter *FioResultWriter
}

// WithDuration sets Duration in TestConfig.
func (t TestConfig) WithDuration(duration time.Duration) TestConfig {
	t.Duration = duration
	return t
}

// WithJob sets Job in TestConfig.
func (t TestConfig) WithJob(job string) TestConfig {
	t.Job = job
	return t
}

// WithJobFile sets JobFile in TestConfig.
func (t TestConfig) WithJobFile(jobFile string) TestConfig {
	t.JobFile = jobFile
	return t
}

// WithPath sets Path in TestConfig.
func (t TestConfig) WithPath(path string) TestConfig {
	t.Path = path
	return t
}

// WithBootDevice sets Path to BootDeviceFioPath in TestConfig.
func (t TestConfig) WithBootDevice() TestConfig {
	t.Path = BootDeviceFioPath
	return t
}

// WithVerifyOnly sets VerifyOnly in TestConfig.
func (t TestConfig) WithVerifyOnly(verifyOnly bool) TestConfig {
	t.VerifyOnly = verifyOnly
	return t
}

// WithResultWriter sets ResultWriter in TestConfig.
func (t TestConfig) WithResultWriter(resultWriter *FioResultWriter) TestConfig {
	t.ResultWriter = resultWriter
	return t
}

// runFIO runs "fio" storage stress with a given config (jobFile), parses the output JSON and returns the result.
// If verifyOnly is true, represents a benchmark collection run.
func runFIO(ctx context.Context, testDataPath, jobFile string, verifyOnly bool) (*fioResult, error) {
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
	result := &fioResult{}
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
	testing.ContextLogf(ctx, "Storage name: %s, info: %v, type: %v, life usage: %d%%",
		info.Name, info.Device, info.Status, info.PercentageUsed)
	return info, nil
}

func validateJob(ctx context.Context, job string) error {
	for _, config := range Configs {
		if job == config {
			return nil
		}
	}
	return errors.Errorf("job = %q, want one of %q", job, Configs)
}

// escapeJSONName replaces all invalid characters that might be present in the device name to appear
// correctly in the perf result JSON.
func escapeJSONName(name string) string {
	return regexp.MustCompile(`[[\]<>{} ]`).ReplaceAllString(name, "_")
}

func resultGroupName(ctx context.Context, res *fioResult, dev string, rawDevTest bool) string {
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

// RunFioStress runs an fio job single given path according to testConfig.
// This function returns an error rather than failing the test.
func RunFioStress(ctx context.Context, testConfig TestConfig) error {
	rawDev := strings.HasPrefix(testConfig.Path, "/dev/")

	if err := validateJob(ctx, testConfig.Job); err != nil {
		return errors.Wrap(err, "failed validating job")
	}

	// Get device status/info.
	info, err := getStorageInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get storage info")
	}
	devName := escapeJSONName(info.Name)

	if !rawDev {
		// Delete the test data file on host.
		defer os.RemoveAll(testConfig.Path)
	}

	testing.ContextLog(ctx, "Running job ", testConfig.Job)
	var res *fioResult
	for start := time.Now().Round(0); ; {
		res, err = runFIO(ctx, testConfig.Path, testConfig.JobFile, testConfig.VerifyOnly)
		if err != nil {
			return errors.Wrapf(err, "%v failed", testConfig.Job)
		}

		// If duration test parameter is 0, we do a single iteration of a test.
		if testConfig.Duration == 0 || time.Since(start) > testConfig.Duration {
			break
		}
	}

	if testConfig.ResultWriter != nil {
		group := resultGroupName(ctx, res, devName, rawDev)
		testConfig.ResultWriter.Report(group, res)
		testConfig.ResultWriter.ReportDiskUsage(group, info.PercentageUsed, info.TotalBytesWritten)
	}

	return nil
}
