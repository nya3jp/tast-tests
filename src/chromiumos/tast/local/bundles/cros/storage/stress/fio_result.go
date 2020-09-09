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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

// FioResult is a serializable structure representing fio results output.
type FioResult struct {
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

// FioResultReport is a result report for a single io run for a group.
type FioResultReport struct {
	Group  string
	Result *FioResult
}

// FioResultWriter is a serial processor of fio results
type FioResultWriter struct {
	perfValues *perf.Values
	resultCh   chan FioResultReport
	doneCh     chan bool
}

func (f *FioResultWriter) writerLoop(ctx context.Context) {
	for {
		report, received := <-f.resultCh
		if received {
			reportResults(ctx, report.Result, report.Group, f.perfValues)
		} else {
			f.doneCh <- true
		}
	}
}

// Start initializes the channels and starts the writer.
func (f *FioResultWriter) Start(ctx context.Context) {
	f.perfValues = perf.NewValues()
	f.resultCh = make(chan FioResultReport)
	f.doneCh = make(chan bool)
	go f.writerLoop(ctx)
}

// StopAndSave finishes the writer and saves the date to the disk.
func (f *FioResultWriter) StopAndSave(path string) {
	close(f.resultCh)
	<-f.doneCh
	close(f.doneCh)

	f.perfValues.Save(path)
	f.perfValues = nil
}

// Report posts a single fio result to the writer.
func (f *FioResultWriter) Report(group string, result *FioResult) {
	f.resultCh <- FioResultReport{group, result}
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

func reportResults(ctx context.Context, res *FioResult, group string, perfValues *perf.Values) {
	for _, job := range res.Jobs {
		if strings.Contains(job.Jobname, "read") || strings.Contains(job.Jobname, "stress") {
			reportJobRWResult(ctx, job.Read, job.Jobname+"_read", group, perfValues)
		}
		if strings.Contains(job.Jobname, "write") || strings.Contains(job.Jobname, "stress") {
			reportJobRWResult(ctx, job.Write, job.Jobname+"_write", group, perfValues)
		}
	}
}
