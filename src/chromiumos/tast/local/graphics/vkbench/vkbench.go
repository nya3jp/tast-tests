// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkbench manipulates the test flow of running vkbench test binaries.
package vkbench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/graphics/glbench"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

var (
	// passRE is a regex to parse test result. It matches a line like
	// "@RESULT:                                  SubmitTest@10 =     221.00 us"
	passRE = regexp.MustCompile(`^@RESULT:\s*(\S+)\s*=\s*(\S+) (\S+)`)
	skipRE = regexp.MustCompile(`^@RESULT:\s*(\S+)\s*=\s*SKIP\[(\S+)\]`)
	failRE = regexp.MustCompile(`^@RESULT:\s*(\S+)\s*=\s*ERROR\[(\S+)\]`)
)

// Config is the interface that setup/runs/teardown the vkbench running environment.
type Config interface {
	SetUp(ctx context.Context) error
	Run(ctx context.Context, preValue interface{}, outDir string) (string, error)
	TearDown(ctx context.Context) error
	IsHasty() bool
}

// Run runs the vkbench binary. outDir specifies the directories to store the results. preValue is the structure given by precondition/fixture for test to access container/environment.
func Run(ctx context.Context, outDir string, fixtValue interface{}, config Config) (resultErr error) {
	// appendErr append the error with msg to resultErr.
	var appendErr = func(err error, msg string, args ...interface{}) error {
		resultErr = errors.Wrap(resultErr, errors.Wrapf(err, msg, args...).Error())
		return resultErr
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(outDir); err != nil {
			appendErr(err, "failed to save perf data")
		}
	}()

	// Leave a bit of time to clean up.
	cleanUpCtx := ctx
	cleanUpTime := 10 * time.Second
	ctx, cancel := ctxutil.Shorten(cleanUpCtx, cleanUpTime)
	defer cancel()

	// Logging the initial machine temperature.
	if err := glbench.ReportTemperature(ctx, pv, "temperature_initial"); err != nil {
		appendErr(err, "failed to report temperature_initial")
	}
	if err := config.SetUp(ctx); err != nil {
		return appendErr(err, "failed to setup vkbench config")
	}
	defer config.TearDown(cleanUpCtx)

	// Only setup benchmark mode if we are not in hasty mode.
	if !config.IsHasty() {
		// Make machine behaviour consistent.
		if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownPreserveUI)); err != nil {
			glbench.SaveFailLog(ctx, filepath.Join(outDir, "before_tests1"))
			testing.ContextLog(ctx, "Unable get cool machine by default setting: ", err)
			if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownConfig{PollTimeout: 1 * time.Minute, PollInterval: 2 * time.Second, CPUTemperatureThreshold: 60000, CoolDownMode: power.CoolDownPreserveUI}); err != nil {
				glbench.SaveFailLog(ctx, filepath.Join(outDir, "before_tests2"))
				appendErr(err, "unable to get cool machine to reach 60C")
			}
		}
	}

	if err := glbench.ReportTemperature(ctx, pv, "temperature_before_test"); err != nil {
		appendErr(err, "failed to log temperature_before_test")
	}

	output, err := config.Run(ctx, fixtValue, outDir)
	if err != nil {
		return appendErr(err, "failed to run glbench")
	}

	// Logging the afterward machine temperature.
	if err := glbench.ReportTemperature(ctx, pv, "temperature_after_test"); err != nil {
		appendErr(err, "failed to report temperature_after_test")
	}

	failedTests, err := analyzeSummary(output, filepath.Join(outDir, "summary.txt"), pv)
	if err != nil {
		return appendErr(err, "failed to analyze summary")
	}
	if len(failedTests) > 0 {
		sort.Strings(failedTests)
		return appendErr(err, "Some images don't match their references: %q; check summary.txt for details", failedTests)
	}
	return
}

// analyzeSummary analyze the output of glbench and write the result to resultPath as well as saving the perf value to pv.
// The function returns the list of failed tests if found.
func analyzeSummary(summary, resultPath string, pv *perf.Values) ([]string, error) {
	// Write a copy of stdout to help debug failures.
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open summary file")
	}
	defer f.Close()

	fmt.Fprintf(f, "%s", summary)
	fmt.Fprintf(f, "==================PostProcessing==================")

	results := strings.Split(summary, "\n")
	if len(results) == 0 {
		return nil, errors.New("no output from test")
	}

	var failedTests []string
	testEnded := false
	for _, line := range results {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@TEST_END") {
			testEnded = true
		}
		if !strings.HasPrefix(line, "@RESULT: ") {
			continue
		}

		if m := passRE.FindStringSubmatch(line); m != nil {
			testName, scoreStr, unit := m[1], m[2], m[3]
			score, err := strconv.ParseFloat(scoreStr, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse score %v", score)
			}
			if score == 0.0 {
				fmt.Fprintf(f, "%s: no score for test.\n", testName)
			}
			pv.Set(perf.Metric{
				Name:      testName,
				Unit:      unit,
				Direction: perf.BiggerIsBetter,
			}, score)
			continue
		}

		if m := skipRE.FindStringSubmatch(line); m != nil {
			testName, msg := m[1], m[2]
			fmt.Fprintf(f, "%s: SKIP[%v].\n", testName, msg)
			continue
		}

		if m := failRE.FindStringSubmatch(line); m != nil {
			testName, msg := m[1], m[2]
			fmt.Fprintf(f, "%s: FAIL[%v].\n", testName, msg)
			failedTests = append(failedTests, testName)
			continue
		}
		return nil, errors.Errorf("failed to recognize line %v", line)
	}
	if !testEnded {
		return nil, errors.New("failed to find end marker")
	}
	return failedTests, nil
}

func saveFailLog(ctx context.Context, dir string) {
	// Create the directory if it is not exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0755)
	}
	faillog.SaveToDir(ctx, dir)
}
