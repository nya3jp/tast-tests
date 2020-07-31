// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/storage"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	defaultFileSize = 1024 * 1024 * 1024
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

// Structure of the relevant entities of fio result JSON.
type fioResult struct {
	Jobs []struct {
		Jobname                 string
		Read, Write, Trim, Sync map[string]interface{}
	}
	DiskUtil []struct {
		Name string
	} `json:"disk_util"`
}

// runFIO runs "fio" storage stress with a given config, parses and returns the output JSON.
func runFIO(ctx context.Context, testDataPath, jobFile string, verify bool) (res fioResult, err error) {
	var result fioResult

	verifyFlag := "0"
	if verify {
		verifyFlag = "1"
	}
	envArgs := []string{
		"FILENAME=" + testDataPath,
		"FILESIZE=" + strconv.Itoa(defaultFileSize),
		"VERIFY_ONLY=" + verifyFlag,
		"CONTINUE_ERRORS=verify",
	}
	extraArgs := []string{"--output-format=json", "--end_fsync=1"}

	cmd := testexec.CommandContext(ctx, "fio", append([]string{jobFile}, extraArgs...)...)
	cmd.Env = envArgs
	testing.ContextLog(ctx, "Environment Arguments: ", envArgs)
	testing.ContextLog(ctx, "Command: ", cmd)
	out, err := cmd.Output()
	// testing.ContextLogf(ctx, "Raw results: %+v\n", string(out))
	if err != nil {
		cmd.DumpLog(ctx)
		testing.ContextLog(ctx, "Failed to write fio running error to log file: ", err)

		// Only append the first line of the output to the error.
		if idx := bytes.IndexByte(out, '\n'); idx != -1 {
			out = out[:idx]
		}
		return result, errors.Wrap(err, "failed to run fio: "+string(out))
	}

	if err := json.Unmarshal(out, &result); err != nil {
		testing.ContextLog(ctx, "Failed to write fio parsing error to log file: ", err)
		return result, errors.Wrap(err, "failed to parse fio result")
	}
	// testing.ContextLogf(ctx, "Result of %s: %+v\n", jobFile, result)
	return result, err
}

func reportJobRWResult(ctx context.Context, testRes map[string]interface{}, prefix, dev string, perfValues *perf.Values) {
	flatResult, err := flattenNestedResults("", testRes)
	if err != nil {
		testing.ContextLog(ctx, "Error flattening results json: ", err)
		return
	}
	// testing.ContextLogf(ctx, "Flattened results: %+v\n", flatResult)

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

func flattenNestedResults(prefix string, nested interface{}) (flat map[string]float64, err error) {
	flat = make(map[string]float64)

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

func merge(to, from map[string]float64) {
	for kt, vt := range from {
		to[kt] = float64(vt)
	}
}

// getDiskSize returns a size string (i.e. "128G") of a storage device.
func getDiskSize(dev string) (sizeGB string, err error) {
	cmd := fmt.Sprintf("cat /proc/partitions | egrep '%v$' | awk '{print $3}'", dev)
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err == nil {
		blocks, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err == nil {
			sizeGB = strconv.Itoa(int(1024*blocks/math.Pow(10, 9.0)+0.5)) + "G"
		}
	}
	return
}

func reportResults(ctx context.Context, s *testing.State, res fioResult, dev string, perfValues *perf.Values) {
	devName := res.DiskUtil[0].Name
	devSize, err := getDiskSize(devName)
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

func getStorageInfo(ctx context.Context, s *testing.State) (info *storage.Info, err error) {
	info, err = storage.Get(ctx)
	if err != nil {
		s.Fatal("Failed to get storage info: ", err)
	}
	if info.Status == storage.Failing {
		s.Error("Storage device is failing, consider removing from DUT farm")
	}
	testing.ContextLogf(ctx, "Storage name: %s, info: %v, type: %v", info.Name, info.Device, info.Status)
	return
}

func validateTestConfig(ctx context.Context, s *testing.State, job string) {
	for _, config := range Configs {
		if job == config {
			return
		}
	}
	s.Fatal("Invalid configuration requested: ", job)
}

func validateJSONName(name string) string {
	r := strings.NewReplacer("<", "_", ">", "_", "{", "_", "}", "_", "[", "_", "]", "_", " ", "_")
	return r.Replace(name)
}

// RunFioStress runs a single given storage job out of the list of supported configs.
func RunFioStress(ctx context.Context, s *testing.State, job string, verify bool) {
	validateTestConfig(ctx, s, job)

	// Get device status/info.
	info, err := getStorageInfo(ctx, s)
	if err != nil {
		s.Fatal("Failed to get storage info: ", err)
	}
	devName := validateJSONName(info.Name)

	// File name the disk I/O test is performed on.
	testDataPath := filepath.Join("/mnt/stateful_partition", "fio_test_data")
	// Delete the test data file on host.
	defer os.RemoveAll(testDataPath)

	perfValues := perf.NewValues()

	testing.ContextLog(ctx, "Running job ", job)
	res, err := runFIO(ctx, testDataPath, s.DataPath(job), verify)
	if err != nil {
		s.Errorf("%v failed: %v", job, err)
	}
	reportResults(ctx, s, res, devName, perfValues)

	perfValues.Save(s.OutDir())
}
