// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// lmbenchPerfPrefix is the performance value prefix
const lmbenchPerfPrefix = "Benchmark.LMBench."

// parseOutput defines the function used to parse lmbench run result.
type parseOutput func(string) (float64, map[string]float64, error)

type commandOpt struct {
	// name is used to identify the run.
	name string
	// option is the command option passed to lmbench executables.
	option []string
}

// runInfo defines the LMbench execution information.
type runInfo struct {
	testName string
	command  string
	parse    parseOutput
	options  []commandOpt
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LMbench,
		Desc:         "Execute LMBench to do benchmark testing and retrieve the results",
		Contacts:     []string{"phuang@cienet.com", "xliu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      setup.BenchmarkChromeFixture,
		Timeout:      30 * time.Minute,
	})
}

func LMbench(ctx context.Context, s *testing.State) {
	perfValues := perf.NewValues()
	// Raw test result.
	results := make(map[string]map[string]float64)

	const (
		bandwidthUnit = "megabytes"
		latencyUnit   = "nanoseconds"
	)

	/* Bandwidth Output examples:
	// 0.001024 961.80
	*/
	parseBandwidthOutput := func(out string) (float64, map[string]float64, error) {
		samplePattern := regexp.MustCompile(`\d+.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return 0.0, nil, errors.Errorf("unable to match time from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))
		f, err := strconv.ParseFloat(matched[1], 64)
		if err != nil {
			return 0.0, nil, errors.Wrapf(err, "failed to parse time %q in IO bandwidth output", matched[1])
		}
		return f, nil, nil
	}

	/* Latency Output examples:
	// stride=128
	// 0.00049 1.182
	// 0.00098 1.182
	// 0.00195 1.182
	// ...
	// 0.01562 1.182
	// 0.02344 1.182
	// 0.03125 1.184
	// ...
	// 48.00000 6.024
	// 64.00000 6.030
	// 128.00000 6.260
	*/
	parseLatencyOutput := func(out string) (float64, map[string]float64, error) {
		samplePattern := regexp.MustCompile(`\d+\.\d+ \d+\.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return 0.0, nil, errors.Errorf("unable to parse latency from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))

		// Put result pairs (array size and latency) into map.
		m := make(map[string]string)
		for _, val := range matched {
			pair := strings.Split(val, " ")
			m[pair[0]] = pair[1]
		}
		// Get the largest array size.
		largest := strings.Split(matched[len(matched)-1], " ")[0]

		// Refer to http://www.bitmover.com/lmbench/lat_mem_rd.8.html
		// INTERPRETING THE OUTPUT section indicates that as a rough guide, you may be able to extract
		// the latencies of the various parts from following array size.
		arraySize := []string{"0.00098", "0.12500", "8.00000", largest}
		// The lentency for the above four array sizes will be written as performance values.
		var latencies []float64
		result := make(map[string]float64)
		for _, s := range arraySize {
			v, ok := m[s]
			if !ok {
				return 0.0, nil, errors.Errorf("failed to obtain latency output for array size %s", s)
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0.0, nil, errors.Wrapf(err, "failed to parse time %q in memory latency output", v)
			}
			latencies = append(latencies, f)
			result[s] = f
		}

		s.Log("Calculate latency geometric mean of ", latencies)
		l, err := calcGeometricMean(latencies)
		if err != nil {
			return 0.0, nil, errors.Wrapf(err, "failed to calculate geometric mean of %v", latencies)
		}
		return l, result, nil
	}

	var bResults []float64
	var lResults []float64

	// Use a large file - chrome executable - as the file to measure file reading bandwidth.
	fileRd := "/opt/google/chrome/chrome"
	frdRun := runInfo{"bw_file_rd", "bw_file_rd", parseBandwidthOutput, []commandOpt{
		// name, and option of each execution
		{"1k", []string{"1k", "io_only", fileRd}},
		{"512k", []string{"512k", "io_only", fileRd}},
		{"1m", []string{"1m", "io_only", fileRd}},
		{"40m", []string{"40m", "io_only", fileRd}},
	}}

	cpRun := runInfo{"bw_mem_cp", "bw_mem", parseBandwidthOutput, []commandOpt{
		{"16k", []string{"16k", "cp"}},
		{"1m", []string{"1m", "cp"}},
		{"256m", []string{"256m", "cp"}},
		{"1024m", []string{"1024m", "cp"}},
	}}
	rdRus := runInfo{"bw_mem_rd", "bw_mem", parseBandwidthOutput, []commandOpt{
		{"16k", []string{"16k", "rd"}},
		{"1m", []string{"1m", "rd"}},
		{"256m", []string{"256m", "rd"}},
		{"1024m", []string{"1024m", "rd"}},
	}}
	wrRun := runInfo{"bw_mem_wr", "bw_mem", parseBandwidthOutput, []commandOpt{
		{"16k", []string{"16k", "wr"}},
		{"1m", []string{"1m", "wr"}},
		{"256m", []string{"256m", "wr"}},
		{"1024m", []string{"1024m", "wr"}},
	}}
	for _, i := range []runInfo{frdRun, cpRun, rdRus, wrRun} {
		val, vals, err := executeBandwidth(ctx, s, i)
		if err != nil {
			s.Fatalf("Failed to execute %s performance test: %v", i.testName, err)
		}
		perfValues.Set(perf.Metric{
			Name:      lmbenchPerfPrefix + i.testName,
			Unit:      bandwidthUnit,
			Direction: perf.BiggerIsBetter,
		}, val)
		results[i.testName] = vals
		bResults = append(bResults, val)
	}
	// Calculate geo mean for all bandwidth values.
	b, err := calcGeometricMean(bResults)
	if err != nil {
		s.Fatalf("Failed to calculate bandwidth geometric mean of bandwidth %v: %v", bResults, err)
	}
	perfValues.Set(perf.Metric{
		Name:      lmbenchPerfPrefix + "BW_GeoMean",
		Unit:      bandwidthUnit,
		Direction: perf.BiggerIsBetter,
	}, b)

	info := runInfo{"lat_mem_rd", "lat_mem_rd", parseLatencyOutput, []commandOpt{
		{"128", []string{"128"}},
	}}
	val, vals, err := executeLatency(ctx, s, info)
	if err != nil {
		s.Fatalf("Failed to execute %s performance test: %v", info.testName, err)
	}
	perfValues.Set(perf.Metric{
		Name:      lmbenchPerfPrefix + info.testName,
		Unit:      latencyUnit,
		Direction: perf.SmallerIsBetter,
	}, val)
	results[info.testName] = vals
	lResults = append(lResults, val)

	// Calculate geo mean for all latency values.
	l, err := calcGeometricMean(lResults)
	if err != nil {
		s.Fatalf("Failed to calculate bandwidth geometric mean of latency %v: %v", lResults, err)
	}
	perfValues.Set(perf.Metric{
		Name:      lmbenchPerfPrefix + "LAT_GeoMean",
		Unit:      latencyUnit,
		Direction: perf.SmallerIsBetter,
	}, l)

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
	filePath := path.Join(s.OutDir(), "lmbench_results.json")
	j, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		s.Fatal("Failed to marshal lmbench data: ", err)
	}
	if err := ioutil.WriteFile(filePath, j, 0644); err != nil {
		s.Fatal("Failed to save lmbench data: ", err)
	}
}

// calcGeometricMean computes the geometric mean but use antilog method to
// prevent overflow: EXP((LOG(x1) + LOG(x2) + LOG(x3)) ... + LOG(xn)) / n)
func calcGeometricMean(scores []float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New("scores can not be empty")
	}
	var mean float64
	for _, score := range scores {
		mean += math.Log(score)
	}
	mean /= float64(len(scores))
	return math.Exp(mean), nil
}

func execute(ctx context.Context, info runInfo, option []string) (float64, map[string]float64, error) {
	// Current version of lmbench on CrOS installs individual benchmarks in /usr/local/bin.
	// It turns out that these lmbench commands print results to stderr. So use CombinedOutput instead of Output() to
	// capture the results. There will be no interleaves between stdout and stderr so we are safe to use CombinedOutput here.
	out, err := testexec.CommandContext(ctx, info.command, option...).CombinedOutput()
	if err != nil {
		return 0.0, nil, errors.Wrapf(err, "failed to run %s benchmark", info.testName)
	}

	val, vals, err := info.parse(string(out))
	if err != nil {
		return 0.0, nil, errors.Wrapf(err, "failed to parse %q output %q", info.testName, string(out))
	}
	return val, vals, err
}

// executeBandwidth does bandwidth tests according to given test info and command options.
func executeBandwidth(ctx context.Context, s *testing.State, info runInfo) (float64, map[string]float64, error) {
	var vals []float64
	runs := make(map[string]float64)
	for _, opt := range info.options {
		s.Logf("Start to run %s benchmark (run %s): %s %s", info.testName, opt.name, info.command, strings.Join(opt.option, " "))
		v, _, err := execute(ctx, info, opt.option)
		if err != nil {
			return 0.0, nil, err
		}
		vals = append(vals, v)
		runs[opt.name] = v
	}
	v, err := calcGeometricMean(vals)
	if err != nil {
		return 0.0, nil, errors.Wrapf(err, "failed to calculate geometric mean of %v", vals)
	}
	return v, runs, nil
}

// executeLatency does latency tests according to given test info and command option.
func executeLatency(ctx context.Context, s *testing.State, info runInfo) (float64, map[string]float64, error) {
	for _, opt := range info.options {
		s.Logf("Start to run %s benchmark (run %s): %s %s", info.testName, opt.name, info.command, strings.Join(opt.option, " "))
		// Only run the first option and return.
		return execute(ctx, info, opt.option)
	}
	return 0.0, nil, errors.New("no run options are given")
}
