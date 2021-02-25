// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/perfutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// benchmarkFilename is a gs bucket file which is used to test file read performance.
const benchmarkFilename = "2160p_60fps_600frames_20181225.h264.mp4"

type parseOutput func(string) (float64, error)

type benchmarkInfo struct {
	command         string
	options         []string
	parse           parseOutput
	performanceUnit string
	direction       perf.Direction
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LMbench,
		Desc:         "Execute LMBench to do benchmark testing and retrieve the results",
		Contacts:     []string{"phuang@cienet.com", "xliu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		HardwareDeps: crostini.CrostiniStable,
		// Var keepState is used by crostini precondition attempting to start the existing VM and container.
		Vars: []string{"keepState"},
		Pre:  crostini.StartedByComponentBuster(),
		Data: []string{
			vm.ArtifactData(),
			crostini.GetContainerMetadataArtifact("buster", false),
			crostini.GetContainerRootfsArtifact("buster", false),
			benchmarkFilename,
		},
		Timeout: 30 * time.Minute,
	})
}

func LMbench(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	perfValues := perf.NewValues()
	defer perfValues.Save(s.OutDir())

	// Prepare error log file.
	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()

	/* Bandwith Output examples:
	// Getting output as following: 0.001024 961.80
	*/
	parseBandwidthBenchOutput := func(out string) (float64, error) {
		samplePattern := regexp.MustCompile(`\d+.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return 0.0, errors.Errorf("unable to match time from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))
		f, err := strconv.ParseFloat(matched[1], 64)
		if err != nil {
			return 0.0, errors.Wrapf(err, "failed to parse time %q in IO bandwidth output", matched[1])
		}
		return f, nil
	}

	/* Latency Output examples:
	// Getting output as following: "stride=128
	// 0.00049 1.182
	// 0.00098 1.182
	// 0.00195 1.182
	// 0.00293 1.182
	// 0.00391 1.182
	// 0.00586 1.182
	// 0.00781 1.182
	// 0.01172 1.182
	// 0.01562 1.182
	// 0.02344 1.182
	// 0.03125 1.184
	// 0.04688 3.546
	// 0.06250 3.546
	// 0.09375 3.545
	// 0.12500 3.603
	// 0.18750 3.635
	// 0.25000 3.884
	// 0.37500 4.018
	// 0.50000 4.098
	// 0.75000 4.094
	// 1.00000 4.106
	// 1.50000 4.106
	// 2.00000 4.206
	// 3.00000 4.446
	// 4.00000 5.154
	// 6.00000 5.611
	// 8.00000 5.813
	// 12.00000 5.894
	// 16.00000 5.938
	// 24.00000 5.992
	// 32.00000 6.001
	// 48.00000 6.024
	// 64.00000 6.030
	*/
	parseLatencyBenchOutput := func(out string) (float64, error) {
		samplePattern := regexp.MustCompile(`\d+.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return 0.0, errors.Errorf("unable to match time from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))

		// Put array size and latency pair into map.
		m := make(map[string]string)
		for index, val := range matched {
			if index%2 == 0 {
				m[val] = matched[index+1]
			}
		}

		// Refer to http://www.bitmover.com/lmbench/lat_mem_rd.8.html
		// INTERPRETING THE OUTPUT section indictes that as a rough guide, you may be able to extract
		// the latencies of the various parts from following array size.
		arraySize := []string{"0.00098", "0.12500", "8.00000", "128.00000"}
		var latencies []float64
		for _, s := range arraySize {
			v, ok := m[s]
			if !ok {
				return 0.0, errors.Errorf("failed to obtain latency output for array size %s", s)
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0.0, errors.Wrapf(err, "failed to parse time %q in memory latency output", v)
			}
			latencies = append(latencies, f)
		}

		s.Log("Calculate latency geometric mean of ", latencies)
		l, err := calcGeometricMean(latencies)
		if err != nil {
			return 0.0, errors.Wrapf(err, "failed to calculate geometric mean of %v", latencies)
		}
		return l, nil
	}

	const (
		bandwidthName = "Benchmark.LMBench.BW_GeoMean"
		bandwidthUnit = "megabytes"
		latencyName   = "Benchmark.LMBench.LAT_GeoMean"
		latencyUnit   = "nanoseconds"
	)
	bRuns := map[string]benchmarkInfo{
		"bw_file_rd": {"bw_file_rd", []string{"42718702", "io_only", s.DataPath(benchmarkFilename)}, parseBandwidthBenchOutput, bandwidthUnit, perf.BiggerIsBetter},
		"bw_mem_cp":  {"bw_mem", []string{"1024m", "cp"}, parseBandwidthBenchOutput, bandwidthUnit, perf.BiggerIsBetter},
		"bw_mem_rd":  {"bw_mem", []string{"1024m", "rd"}, parseBandwidthBenchOutput, bandwidthUnit, perf.BiggerIsBetter},
		"bw_mem_wr":  {"bw_mem", []string{"1024m", "wr"}, parseBandwidthBenchOutput, bandwidthUnit, perf.BiggerIsBetter},
	}
	var bResults []float64
	for commandName, info := range bRuns {
		f, err := testBandwidthAndLatency(ctx, s, errFile, cont, perfValues, commandName, info)
		if err != nil {
			s.Fatalf("Failed to get %s performance value: %v", commandName, err)
		}
		bResults = append(bResults, f)
	}
	b, err := calcGeometricMean(bResults)
	if err != nil {
		s.Fatalf("Failed to calculate bandwidth geometric mean of %v: %v", bResults, err)
	}
	perfValues.Set(perf.Metric{
		Name:      bandwidthName,
		Unit:      bandwidthUnit,
		Direction: perf.BiggerIsBetter,
	}, b)

	lRuns := map[string]benchmarkInfo{
		"lat_mem_rd": {"lat_mem_rd", []string{"128"}, parseLatencyBenchOutput, latencyUnit, perf.SmallerIsBetter},
	}
	var lResults []float64
	for commandName, info := range lRuns {
		f, err := testBandwidthAndLatency(ctx, s, errFile, cont, perfValues, commandName, info)
		if err != nil {
			s.Fatalf("Failed to get %s performance value: %v", commandName, err)
		}
		lResults = append(lResults, f)
	}
	l, err := calcGeometricMean(lResults)
	if err != nil {
		s.Fatalf("Failed to calculate latency geometric mean of %v: %v", lResults, err)
	}
	perfValues.Set(perf.Metric{
		Name:      latencyName,
		Unit:      latencyUnit,
		Direction: perf.SmallerIsBetter,
	}, l)
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

// testBandwidthAndLatency does tests according to given test info.
func testBandwidthAndLatency(ctx context.Context, s *testing.State, errFile *os.File, cont *vm.Container, perfValues *perf.Values, commandName string, info benchmarkInfo) (float64, error) {
	// Latest lmbench defaults to install individual microbenchamrks in /usr/lib/lmbench/bin/<arch dependent folder>
	// (e.g., /usr/lib/lmbench/bin/x86_64-linux-gnu). So needs to find the exact path.
	out, err := perfutil.RunCmd(ctx, cont.Command(ctx, "find", "/usr/lib/lmbench", "-name", commandName), errFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to find %s benchmark binary in container", commandName)
	}
	commandBenchBinary := strings.TrimSpace(string(out))
	s.Logf("Found %s benchmark installed in container: %s", commandName, commandBenchBinary)

	s.Logf("Start to run benchmark %s", commandName)

	// Current version of lmbench on CrOS installs individual benchmarks in /usr/local/bin so
	// can be called directly.
	// Usage: lat_mem_rd [-P <parallelism>] [-W <warmup>] [-N <repetitions>] [-t] len [stride...]
	// Usage: bw_file_rd [-C] [-P <parallelism>] [-W <warmup>] [-N <repetitions>] <size> open2close|io_only <filename> ... min size=64
	out, err = perfutil.RunCmd(ctx, testexec.CommandContext(ctx, info.command, info.options...), errFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to run %q on host", commandName)
	}
	s.Log("Get output and parse benchmark")

	val, err := info.parse(string(out))
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse %q output on host", commandName)
	}
	s.Logf("Prepare performance metric data %q %v: writing value %.2f", commandName, info.options, val)
	perfValues.Set(perf.Metric{
		Name:      "Benchmark.LMBench." + commandName,
		Unit:      info.performanceUnit,
		Direction: info.direction,
	}, val)
	return val, nil
}
