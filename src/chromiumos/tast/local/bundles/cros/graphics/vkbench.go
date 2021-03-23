// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type vkbenchConfig struct {
	hasty bool // hasty indicates to run vkbench in hasty mode.
}

const (
	vkbenchPath       = "/usr/local/graphics/vkbench/vkbench"
	vkbenchShaderPath = "/usr/local/graphics/vkbench/shaders"
)

var (
	// vkbenchRE is a regex to parse test result. It matches a line like
	// "@RESULT:                                  SubmitTest@10 =     221.00 us"
	vkbenchRE = regexp.MustCompile(`^@RESULT:\s*(\S+)\s*=\s*(\S+) (\S+)`)
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VKBench,
		Desc: "Run vkbench (a benchmark that times graphics intensive activities for vulkan), check results and report its performance",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		SoftwareDeps: []string{"no_qemu", "vulkan"},
		Params: []testing.Param{{
			Name:      "",
			Val:       vkbenchConfig{hasty: false},
			ExtraAttr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
			Timeout:   5 * time.Minute,
		}, {
			Name:      "hasty",
			Val:       vkbenchConfig{hasty: true},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   5 * time.Minute,
		}},
		Fixture: "gpuWatchHangs",
	})
}

// logTemperature logs the current temperature reading.
func logTemperature(ctx context.Context) {
	temp, err := sysutil.TemperatureInputMax()
	if err != nil {
		testing.ContextLog(ctx, "Can't read maximum temperature: ", err)
	}
	testing.ContextLog(ctx, "Temperature: ", temp)
}

// VKBench benchmarks the vulkan performance.
func VKBench(ctx context.Context, s *testing.State) {
	testConfig := s.Param().(vkbenchConfig)

	logTemperature(ctx)
	if !testConfig.hasty {
		if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
			s.Log("Unable get cool machine. Trying to get idle cpu: ", err)
			if err2 := cpu.WaitUntilIdle(ctx); err2 != nil {
				s.Error("Unable to get stable machine: ", errors.Wrap(err, err2.Error()))
			}
		}
	}

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed on set up: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	var cmd *testexec.Cmd
	args := []string{"--spirv_dir", vkbenchShaderPath, "--out_dir", filepath.Join(s.OutDir(), "vkbench")}
	if testConfig.hasty {
		args = append(args, "--hasty")
	}
	cmd = testexec.CommandContext(ctx, vkbenchPath, args...)
	s.Log("Running ", shutil.EscapeSlice(cmd.Args))
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run test: ", err)
	}
	summary := string(b)

	logTemperature(ctx)
	resultPath := filepath.Join(s.OutDir(), "summary.txt")
	pv, err := analyzeResult(ctx, resultPath, summary)
	if err != nil {
		s.Fatal("Failure analyzing result: ", err)
	}

	if !testConfig.hasty {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}
}

// analyzeResult analyze the summary and returns the perf value.
func analyzeResult(ctx context.Context, resultPath, summary string) (*perf.Values, error) {
	pv := perf.NewValues()
	// Write a copy of stdout to help debug failures.
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to save summary file")
	}
	defer f.Close()

	fmt.Fprintf(f, "%s", summary)
	fmt.Fprintf(f, "==================PostProcessing==================")

	results := strings.Split(summary, "\n")
	if len(results) == 0 {
		return nil, errors.New("no output from test")
	}

	testEnded := false
	for _, line := range results {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@TEST_END") {
			testEnded = true
		}

		if !strings.HasPrefix(line, "@RESULT: ") {
			continue
		}

		m := vkbenchRE.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.Errorf("line `%q` mismatch with regex `%q`", line, vkbenchRE.String())
		}
		testName, scoreStr, unit := m[1], m[2], m[3]
		score, err := strconv.ParseFloat(scoreStr, 32)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse score")
		}

		errMsg := ""
		if score == 0.0 {
			errMsg = "no score for test."
		}

		if errMsg != "" {
			fmt.Fprintf(f, "%s: %s\n", testName, errMsg)
		}
		// replace @ with - as perf package doesn't accept the character.
		testName = strings.ReplaceAll(testName, "@", "-")
		pv.Set(perf.Metric{
			Name:      testName,
			Unit:      unit,
			Direction: perf.BiggerIsBetter,
		}, score)
	}

	if !testEnded {
		return nil, errors.New("failed to find end marker")
	}
	return pv, nil
}
