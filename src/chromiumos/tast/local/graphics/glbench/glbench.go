// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package glbench manipulates the test flow of running glbench test binaries.
package glbench

import (
	"context"
	"fmt"
	"io/ioutil"
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
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

var (
	// glbench installation folder.
	glbenchDir = "/usr/local/glbench/"
	// referenceImageFile contains good images.
	referenceImageFile = filepath.Join(glbenchDir, "files/glbench_reference_images.txt")
	// knownBadImagesFile contains images that are bad but for which the bug has not been fixed yet.
	knownBadImagesFile = filepath.Join(glbenchDir, "files/glbench_knownbad_images.txt")
	// fixedBadImagesFile contains images that are bad and for which a fix has been submitted.
	fixedBadImagesFile = filepath.Join(glbenchDir, "files/glbench_fixedbad_images.txt")

	// resultRE is a regex to parse test result. It matches a line like
	// "@RESULT: swap_swap                    =   214.77 us           [swap_swap.pixmd5-20dbc406b95e214a799a6a7f9c700d2f.png]" .
	resultRE = regexp.MustCompile(`^@RESULT: (\S+)\s*=\s*(\S+) (\S+)\s*\[(.+)\]`)
)

// Config is the interface that setup/runs/teardown the glbench running environment.
type Config interface {
	SetUp(ctx context.Context) error
	Run(ctx context.Context, preValue interface{}, outDir string) (string, error)
	TearDown(ctx context.Context) error
	IsHasty() bool
}

// Run runs the glbench binary. outDir specifies the directories to store the results. preValue is the structure given by precondition/fixture for test to access container/environment.
func Run(ctx context.Context, outDir string, preValue interface{}, config Config) (resultErr error) {
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

	if err := config.SetUp(ctx); err != nil {
		return appendErr(err, "failed to setup glbench config")
	}
	defer config.TearDown(cleanUpCtx)

	// Logging the initial machine temperature.
	if err := ReportTemperature(ctx, pv, "temperature_1_start"); err != nil {
		appendErr(err, "failed to report temperature")
	}
	// Only setup benchmark mode if we are not in hasty mode.
	if !config.IsHasty() {
		// Make machine behaviour consistent.
		if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownPreserveUI)); err != nil {
			SaveFailLog(ctx, filepath.Join(outDir, "before_tests1"))
			testing.ContextLog(ctx, "Unable get cool machine by default setting: ", err)
			if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownConfig{PollTimeout: 1 * time.Minute, PollInterval: 2 * time.Second, CPUTemperatureThreshold: 60000, CoolDownMode: power.CoolDownPreserveUI}); err != nil {
				SaveFailLog(ctx, filepath.Join(outDir, "before_tests2"))
				appendErr(err, "unable to get cool machine to reach 60C")
			}
		}
	}

	// config.Run should run the
	output, err := config.Run(ctx, preValue, outDir)
	if err != nil {
		return appendErr(err, "failed to run glbench")
	}

	// Logging the afterward machine temperature.
	if err := ReportTemperature(ctx, pv, "temperature_3_after_test"); err != nil {
		appendErr(err, "failed to report temperature")
	}

	failedTests, err := analyzeSummary(output, filepath.Join(outDir, "summary.txt"), pv)
	if err != nil {
		return appendErr(err, "failed to write summary")
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

	fmt.Fprintf(f, `# ---------------------------------------------------
#
%s

# -------------------------------------------------
# [glbench.go postprocessing]
`, summary)

	// Analyze the output. Sample:
	// # board_id: NVIDIA Corporation - Quadro FX 380/PCI/SSE2
	// # Running: ../glbench -save -outdir=img
	// @RESULT: swap_swap = 221.36 us [swap_swap.pixmd5-20dbc...f9c700d2f.png]
	results := strings.Split(summary, "\n")
	if len(results) == 0 {
		return nil, errors.New("no output from test")
	}

	readFile := func(f string) (string, error) {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return "", errors.Wrap(err, "failed to read files")
		}
		return string(b), nil
	}
	// The good images, the silenced and the zombie/recurring failures.
	referenceImageNames, err := readFile(referenceImageFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed ot read referenceImageFile")
	}
	knownBadImageNames, err := readFile(knownBadImagesFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed ot read knownBadImagesFile")
	}
	fixedBadImageNames, err := readFile(fixedBadImagesFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed ot read fixedBadImagesFile")
	}

	// Check if we saw GLBench end as expected (without crashing).
	testEndedNormal := false
	for _, line := range results {
		if strings.HasPrefix(strings.TrimSpace(line), "@TEST_END") {
			testEndedNormal = true
		}
	}
	if !testEndedNormal {
		return nil, errors.Wrap(err, "no end marker(presume crash/missing images)")
	}

	// Analyze individual test results in summary.
	var failedTests []string
	for _, line := range results {
		line := strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@RESULT: ") {
			continue
		}
		m := resultRE.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.Errorf("%q unexpectedly didn't match %q", line, resultRE.String())
		}

		testName, score, unit, imageFile := m[1], m[2], m[3], m[4]
		testRating, err := strconv.ParseFloat(score, 32)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse score")
		}

		// Prepend unit to test name to maintain backwards compatibility with existing data.
		perfValueName := fmt.Sprintf("%s_%s", unit, testName)
		pv.Set(perf.Metric{
			Name:      perfValueName,
			Variant:   perfValueName,
			Unit:      unit,
			Direction: perf.BiggerIsBetter,
		}, testRating)

		errMsg := ""
		// Classify result image.
		switch {
		case testRating == -1.0:
			// Test generated GL Error.
			glError := strings.Split(imageFile, "=")[1]
			errMsg = fmt.Sprintf("GLError %s during test", glError)
			failedTests = append(failedTests, testName)
		case testRating == 0.0:
			// Tests for which glbench does not generate a meaningful perf score.
			errMsg = "no score for test"
		case strings.Contains(fixedBadImageNames, imageFile):
			// We know the image looked bad at some point in time but we thought
			// it was fixed. Throw an exception as a reminder.
			errMsg = fmt.Sprintf("fixedbad [%s]", imageFile)
			failedTests = append(failedTests, testName)
		case strings.Contains(knownBadImageNames, imageFile):
			// We have triaged the failure and have filed a tracking bug.
			// Don't throw an exception and remind there is a problem.
			errMsg = fmt.Sprintf("knownbad [%s]", imageFile)
			// This failure is allowed so don't add to failedTests.
		case strings.Contains(referenceImageNames, imageFile):
			// Known good reference images (default).
		case imageFile == "none":
			// Tests that do not write images can't fail because of them.
		case noChecksumTest(testName):
			// TODO(ihf): these really should not write any images
		default:
			// Completely unknown images. Report a failure.
			errMsg = fmt.Sprintf("unknown [%s]", imageFile)
			failedTests = append(failedTests, testName)
		}

		if errMsg != "" {
			fmt.Fprintf(f, "# %s: %s\n", testName, errMsg)
		}
	}
	return failedTests, nil
}

// ReportTemperature set the current temperature to pv. If there's problem reading the value, it sets -1000 as the temperature.
func ReportTemperature(ctx context.Context, pv *perf.Values, name string) error {
	temp, err := sysutil.TemperatureInputMax()
	if err != nil {
		temp = -1000.0
		testing.ContextLog(ctx, "Can't read maximum temperature: ", err)
	}
	pv.Set(perf.Metric{
		Name:      name,
		Unit:      "Celsius",
		Direction: perf.SmallerIsBetter,
	}, temp)
	return nil
}

// noChecksumTests are tests that do not draw anything.
// They can only be used to check performance.
var noChecksumTests = []string{
	"compositing_no_fill",
	"pixel_read",
	"texture_rebind_rgba_teximage2d",
	"texture_reuse_luminance_teximage2d",
	"texture_reuse_luminance_texsubimage2d",
	"texture_reuse_rgba_teximage2d",
	"texture_reuse_rgba_texsubimage2d",
	"context_glsimple",
	"swap_glsimple",
}

// noChecksumTest checks if given test requires no screenshot checksum.
func noChecksumTest(name string) bool {
	for _, x := range noChecksumTests {
		if strings.HasPrefix(name, x) {
			return true
		}
	}
	return false
}

// SaveFailLog actively calls faillog.SaveToDir to save information for future debugging.
func SaveFailLog(ctx context.Context, dir string) {
	// Create the directory if it is not exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0755)
	}
	faillog.SaveToDir(ctx, dir)
}
