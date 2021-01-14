// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

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

// fioResultReport is a result report for a single io run for a group.
type fioResultReport struct {
	group  string
	result *fioResult
}

// fioDiskUsageReport is a report of disk lifetime usage.
type fioDiskUsageReport struct {
	name           string
	percentageUsed int64
	bytesWritten   int64
}

// FioResultWriter is a serial processor of fio results.
type FioResultWriter struct {
	resultLock sync.Mutex
	results    []fioResultReport
	diskUsages []fioDiskUsageReport
}

// Save processes and saves reported results.
func (f *FioResultWriter) Save(ctx context.Context, path string, writeKeyVal bool) {
	f.resultLock.Lock()
	defer f.resultLock.Unlock()

	perfValues := perf.NewValues()

	for _, report := range f.results {
		reportResults(ctx, report.result, report.group, perfValues)
	}

	for _, disk := range f.diskUsages {
		reportDiskPercentageUsed(ctx, disk, perfValues)
	}
	perfValues.Save(path)

	if writeKeyVal {
		for _, report := range f.results {
			values := saveToKeyVal(ctx, report.result, report.group)
			if err := WriteKeyVals(path, values); err != nil {
				testing.ContextLog(ctx, "Error writing results to keyval file: ", err)
			}
		}
	}

	f.results = nil
}

// Report posts a single fio result to the writer.
func (f *FioResultWriter) Report(group string, result *fioResult) {
	f.resultLock.Lock()
	defer f.resultLock.Unlock()
	f.results = append(f.results, fioResultReport{group, result})
}

// ReportDiskUsage records the disk usage percents to report it at save time.
func (f *FioResultWriter) ReportDiskUsage(diskName string, percentageUsed, totalBytesWritten int64) {
	if len(diskName) == 0 || percentageUsed == -1 {
		return // No reporting of empty or wrong disk usage.
	}

	f.resultLock.Lock()
	defer f.resultLock.Unlock()
	f.diskUsages = append(f.diskUsages, fioDiskUsageReport{diskName, percentageUsed, totalBytesWritten})
}

func reportDiskPercentageUsed(ctx context.Context, diskUsage fioDiskUsageReport, perfValues *perf.Values) {
	perfValues.Set(perf.Metric{
		Name:      "disk_percentage_used",
		Variant:   diskUsage.name,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, float64(diskUsage.percentageUsed))

	perfValues.Append(perf.Metric{
		Name:      "total_bytes_written",
		Variant:   diskUsage.name,
		Unit:      "byte",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, float64(diskUsage.bytesWritten))
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
// TODO(abergman): can we avoid creating map for each nest level?
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
// TODO(abergman): Could we use gopsutil?
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

func reportResults(ctx context.Context, res *fioResult, group string, perfValues *perf.Values) {
	for _, job := range res.Jobs {
		reportJobRWResult(ctx, job.Read, job.Jobname+"_read", group, perfValues)
		reportJobRWResult(ctx, job.Write, job.Jobname+"_write", group, perfValues)
	}
}

func extractResultValues(ctx context.Context, testRes map[string]interface{}, prefix, dev string, values map[string]float64) {
	flatResult, err := flattenNestedResults("", testRes)
	if err != nil {
		testing.ContextLog(ctx, "Error flattening results json: ", err)
		return
	}

	for k, v := range flatResult {
		name := "_" + prefix + k + "{perf}"
		values[name] = v
	}
}

func saveToKeyVal(ctx context.Context, res *fioResult, group string) (values map[string]float64) {
	values = make(map[string]float64)

	for _, job := range res.Jobs {
		extractResultValues(ctx, job.Read, job.Jobname+"_read", group, values)
		extractResultValues(ctx, job.Write, job.Jobname+"_write", group, values)
		extractResultValues(ctx, job.Trim, job.Jobname+"_trim", group, values)
		extractResultValues(ctx, job.Sync, job.Jobname+"_sync", group, values)
	}

	return values
}
