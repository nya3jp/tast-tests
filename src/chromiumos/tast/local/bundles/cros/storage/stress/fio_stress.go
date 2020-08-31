// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
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

	// PerfValues is a perf values collection to report the results into.
	PerfValues *perf.Values
}

// fioResult is a serializable structure representing fio results output.
type fioResult struct {
	Jobs []struct {
		Jobname string                 `json:"jobname"`
		Read    map[string]interface{} `json:"read"`
		Write   map[string]interface{} `json:"write"`
		Trim    map[string]interface{} `json:"trim"`
		Sync    map[string]interface{} `json:"sync"`
	}
	DiskUtil []struct {
		Name string
	} `json:"disk_util"`
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
		"CONTINUE_ERRORS=verifyOnly",
	}
	testing.ContextLog(ctx, "Environment: ", cmd.Env)
	testing.ContextLog(ctx, "Running command: ", cmd)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run fio")
	}
	var result fioResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse fio result")
	}
	return &result, nil
}

// reportJobRWResult appends metrics for latency and bandwidth from the current test results
// to the given perf values.
func reportJobRWResult(ctx context.Context, testRes map[string]interface{}, prefix, dev string, perfValues *perf.Values) {
	flatResult, err := flattenNestedResults("", testRes)
	if err != nil {
		testing.ContextLog(ctx, "Error flattening results json: ", err)
		return
	}

	for k, v := range flatResult {
		if strings.Contains(k, "percentile") {
			perfValues.Append(perf.Metric{
				Name:      "_" + prefix + k,
				Variant:   dev,
				Unit:      "ns",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, v)
		} else if k == "_bw" {
			perfValues.Append(perf.Metric{
				Name:      "_" + prefix + k,
				Variant:   dev,
				Unit:      "KB_per_sec",
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			}, v)
		}
	}
}

// flattenNestedResults flattens nested structures to the root level.
// Names are prefixed with the nested names, i.e. {foo: { bar: {}}} -> {foo<prefix>bar: {}}
func flattenNestedResults(prefix string, nested interface{}) (flat map[string]float64, err error) {
	flat = make(map[string]float64)

	merge := func(to, from map[string]float64) {
		for kt, vt := range from {
			to[kt] = float64(vt)
		}
	}

	switch nested := nested.(type) {
	case map[string]interface{}:
		for k, v := range nested {
			fm1, fe := flattenNestedResults(prefix+"_"+k, v)
			if fe != nil {
				err = fe
				return
			}
			merge(flat, fm1)
		}
	case []interface{}:
		for i, v := range nested {
			fm1, fe := flattenNestedResults(prefix+"_"+strconv.Itoa(i), v)
			if fe != nil {
				err = fe
				return
			}
			merge(flat, fm1)
		}
	default:
		flat[prefix] = nested.(float64)
	}
	return
}

// diskSizePretty returns a size string (i.e. "128G") of a storage device.
func diskSizePretty(dev string) (sizeGB string, err error) {
	cmd := fmt.Sprintf("cat /proc/partitions | egrep '%v$' | awk '{print $3}'", dev)
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", err
	}
	blocks, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return "", err
	}
	// Covert number of blocks to bytes (*1024), then to a string of whole GB,
	// i.e. 125034840 -> "128G"
	return strconv.Itoa(int(1024*blocks/math.Pow(10, 9.0)+0.5)) + "G", nil
}

func reportResults(ctx context.Context, res *fioResult, dev string, perfValues *perf.Values) {
	devName := res.DiskUtil[0].Name
	devSize, err := diskSizePretty(devName)
	if err != nil {
		testing.ContextLog(ctx, "Error acquiring size of device: ", devName, err)
		devSize = ""
	}
	testing.ContextLogf(ctx, "Size of device: %s is: %s", devName, devSize)
	group := dev + "_" + string(devSize)

	for _, job := range res.Jobs {
		if strings.Contains(job.Jobname, "read") || strings.Contains(job.Jobname, "stress") {
			reportJobRWResult(ctx, job.Read, job.Jobname+"_read", group, perfValues)
		}
		if strings.Contains(job.Jobname, "write") || strings.Contains(job.Jobname, "stress") {
			reportJobRWResult(ctx, job.Write, job.Jobname+"_write", group, perfValues)
		}
	}
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

// RunFioStress runs a single given storage job. job must be in Configs.
// If verifyOnly is true, only benchmark data is collected to result-chart.json
func RunFioStress(ctx context.Context, s *testing.State, job string, testConfig *TestConfig) {
	if testConfig == nil {
		testConfig = &TestConfig{
			VerifyOnly: false,
		}
	}

	validateTestConfig(ctx, s, job)

	// Get device status/info.
	info, err := getStorageInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get storage info: ", err)
	}
	devName := escapeJSONName(info.Name)

	// File name the disk I/O test is performed on.
	const testDataPath = "/mnt/stateful_partition/fio_test_data"
	// Delete the test data file on host.
	defer os.RemoveAll(testDataPath)

	testing.ContextLog(ctx, "Running job ", job)
	var res *fioResult
	for start := time.Now(); ; {
		res, err = runFIO(ctx, testDataPath, s.DataPath(job), testConfig.VerifyOnly)
		if err != nil {
			s.Errorf("%v failed: %v", job, err)
		}

		// If duration test parameter is 0, we do a single iteration of a test.
		if testConfig.Duration == 0 || time.Now().Sub(start) > testConfig.Duration {
			break
		}
	}

	if testConfig.PerfValues != nil {
		reportResults(ctx, res, devName, testConfig.PerfValues)
	}
}
