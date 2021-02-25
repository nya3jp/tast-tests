// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
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

type parseOutput func(string) ([]float64, error)

type benchmarkInfo struct {
	options         []string
	parse           parseOutput
	performanceName string
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
		// Var keepState is used by crostini precondtion attempting to start the existing VM and container.
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

	testLmbench(ctx, s, errFile, cont, perfValues)
}

func testLmbench(ctx context.Context, s *testing.State, errFile *os.File, cont *vm.Container, perfValues *perf.Values) {
	/* Output examples:
	// Getting output as following: 0.001024 961.80
	*/
	parseBandwidthBenchOutput := func(out string) (bandwidth []float64, err error) {
		samplePattern := regexp.MustCompile(`\d+.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return bandwidth, errors.Errorf("unable to match time from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))
		f, err := strconv.ParseFloat(matched[1], 64)
		if err != nil {
			return bandwidth, errors.Wrapf(err, "failed to parse time %q in IO bandwidth output", matched[1])
		}
		bandwidth = append(bandwidth, f)
		return bandwidth, nil
	}

	/* Output examples:
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
	parseLatencyBenchOutput := func(out string) (latency []float64, err error) {
		samplePattern := regexp.MustCompile(`\d+.\d+`)
		matched := samplePattern.FindAllString(strings.TrimSpace(out), -1)
		if matched == nil {
			return latency, errors.Errorf("unable to match time from %q", out)
		}
		s.Logf("Found matched: %s", strings.Join(matched[:], ", "))

		for index, val := range matched {
			switch val {
			case "0.00098", "0.12500", "8.00000", "128.00000":
				f, err := strconv.ParseFloat(matched[index+1], 64)
				if err != nil {
					return latency, errors.Wrapf(err, "failed to parse time %q in memory latency output", matched[index+1])
				}
				latency = append(latency, f)
			}
		}

		return latency, nil
	}

	runs := map[string]benchmarkInfo{
		"bw_file_rd": {[]string{"42718702", "io_only", s.DataPath(benchmarkFilename)}, parseBandwidthBenchOutput, "Benchmark.LMBench.BW_GeoMean", "megabytes", perf.BiggerIsBetter},
		// TODO: add following test:
		//"bw_mem_cp": {""},
		//"bw_mem_rd": {""},
		//"bw_mem_wr": {""}
		"lat_mem_rd": {[]string{"128"}, parseLatencyBenchOutput, "Benchmark.LMBench.LAT_GeoMean", "nanoseconds", perf.SmallerIsBetter},
	}
	for commandName, info := range runs {
		testBandwidthAndLatency(ctx, s, errFile, cont, perfValues, commandName, info)
	}
}

// testBandwidthAndLatency does tests according to given test info.
func testBandwidthAndLatency(ctx context.Context, s *testing.State, errFile *os.File, cont *vm.Container, perfValues *perf.Values, commandName string, info benchmarkInfo) {
	// Latest lmbench defaults to install individual microbenchamrks in /usr/lib/lmbench/bin/<arch dependent folder>
	// (e.g., /usr/lib/lmbench/bin/x86_64-linux-gnu). So needs to find the exact path.
	out, err := perfutil.RunCmd(ctx, cont.Command(ctx, "find", "/usr/lib/lmbench", "-name", commandName), errFile)
	if err != nil {
		s.Fatalf("Failed to find %s benchmark binary in container: %v", commandName, err)
	}
	commandBenchBinary := strings.TrimSpace(string(out))
	s.Logf("Found %s benchmark installed in container: %s", commandName, commandBenchBinary)

	// Measure time.
	measureTime := func(name string, info benchmarkInfo) error {
		// Current version of lmbench on CrOS installs individual benchmarks in /usr/local/bin so
		// can be called directly.
		// Usage: lat_mem_rd [-P <parallelism>] [-W <warmup>] [-N <repetitions>] [-t] len [stride...]
		// Usage: bw_file_rd [-C] [-P <parallelism>] [-W <warmup>] [-N <repetitions>] <size> open2close|io_only <filename> ... min size=64
		out, err := perfutil.RunCmd(ctx, testexec.CommandContext(ctx, name, info.options...), errFile)
		if err != nil {
			return errors.Wrapf(err, "failed to run %s on host", name)
		}
		s.Log("Get output and parse benchmark")

		list, err := info.parse(string(out))
		if err != nil {
			return errors.Wrapf(err, "failed to parse %s output on host", name)
		}

		s.Logf("Getting performance list: %s", list)
		for _, val := range list {
			// Output.
			s.Logf("%s %v: writing value %.2f", name, info.options, val)

			perfValues.Set(perf.Metric{
				Name:      info.performanceName,
				Unit:      info.performanceUnit,
				Direction: info.direction,
			}, val)
		}

		return nil
	}

	s.Logf("Start to run benchmark %s", commandName)
	if err := measureTime(commandName, info); err != nil {
		s.Errorf("Failed to measure time for command %v: %v", commandName, err)
	}
}
