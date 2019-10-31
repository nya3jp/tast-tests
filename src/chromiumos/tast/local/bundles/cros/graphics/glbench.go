// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GLBench,
		Desc: "Run glbench (a benchmark that times graphics intensive activities), check results and report its performance",
		Contacts: []string{
			"andrescj@chromium.org",
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
			"oka@chromium.org", // Tast port.
		},
		Params: []testing.Param{
			{
				Name:      "",
				Val:       false,
				Timeout:   1 * time.Hour,
				ExtraAttr: []string{"group:graphics", "graphics_nightly"},
			},
			{
				Name:      "hasty",
				Val:       true,
				ExtraAttr: []string{"group:mainline", "informational"},
			},
		},
		SoftwareDeps: []string{"no_qemu"},
	})
}

const glbenchDir = "/usr/local/glbench/"

var (
	// glbench is the executable for performance testing.
	glbench = filepath.Join(glbenchDir, "bin/glbench")

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

func GLBench(ctx context.Context, s *testing.State) {
	hasty := s.Param().(bool)
	// If UI is running, we must stop it and restore later.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed on set up: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()

	var pv *perf.Values // nil when hasty == true
	if !hasty {
		pv = perf.NewValues()
		defer func() {
			if err := pv.Save(s.OutDir()); err != nil {
				s.Error("Failed to save perf data: ", err)
			}
		}()

		must := func(err error) {
			if err != nil {
				s.Fatal("Set up failed: ", err)
			}
		}

		must(reportTemperatureCritical(ctx, pv, "temperature_critical"))
		must(reportTemperature(pv, "temperature_1_start"))

		cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
		must(err)
		defer cleanUpBenchmark(ctx)

		// Leave a bit of time to clean up benchmark mode.
		cleanUpTime := 10 * time.Second
		var cancel func()
		ctx, cancel = ctxutil.Shorten(ctx, cleanUpTime)
		defer cancel()

		// Make machine behaviour consistent.
		must(cpu.WaitUntilIdle(ctx))
		must(reportTemperature(pv, "temperature_2_before_test"))
	}

	args := []string{"-save", "-outdir=" + s.OutDir()}
	if hasty {
		args = append(args, "-hasty")
	}
	// Run the test, saving is optional and helps with debugging
	// and reference image management. If unknown images are
	// encountered one can take them from the outdir and copy
	// them (after verification) into the reference image dir.
	cmd := testexec.CommandContext(ctx, glbench, args...)
	cmdLine := shutil.EscapeSlice(cmd.Args)

	s.Log("Running ", cmdLine)
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run test: ", err)
	}
	summary := string(b)

	if pv != nil {
		if err := reportTemperature(pv, "temperature_3_after_test"); err != nil {
			s.Fatal("Failed after benchmark run: ", err)
		}
	}

	// Write a copy of stdout to help debug failures.
	resultPath := filepath.Join(s.OutDir(), "summary.txt")
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal("Cannot check summary: ", err)
	}
	defer f.Close()
	fmt.Fprintf(f, `# ---------------------------------------------------
# [%s]
%s

# -------------------------------------------------
# [glbench.go postprocessing]
`, cmdLine, summary)

	// Analyze the output. Sample:
	// # board_id: NVIDIA Corporation - Quadro FX 380/PCI/SSE2
	// # Running: ../glbench -save -outdir=img
	// @RESULT: swap_swap = 221.36 us [swap_swap.pixmd5-20dbc...f9c700d2f.png]
	results := strings.Split(summary, "\n")
	if len(results) == 0 {
		s.Fatal("No output from test")
	}

	readFile := func(f string) string {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			s.Fatal("Failed to read image names: ", err)
		}
		return string(b)
	}
	// The good images, the silenced and the zombie/recurring failures.
	referenceImageNames := readFile(referenceImageFile)
	knownBadImageNames := readFile(knownBadImagesFile)
	fixedBadImageNames := readFile(fixedBadImagesFile)

	// Check if we saw GLBench end as expected (without crashing).
	testEndedNormal := false
	for _, line := range results {
		if strings.HasPrefix(strings.TrimSpace(line), "@TEST_END") {
			testEndedNormal = true
		}
	}
	if !testEndedNormal {
		s.Error("No end marker; presume crash/missing images")
	}

	// Analyze individual test results in summary.
	failedTests := make(map[string]string)

	for _, line := range results {
		line := strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@RESULT: ") {
			continue
		}
		m := resultRE.FindStringSubmatch(line)
		if m == nil {
			s.Fatalf("%q unexpectedly didn't match %q", line, resultRE.String())
		}

		testName, score, unit, imageFile := m[1], m[2], m[3], m[4]
		testRating, err := strconv.ParseFloat(score, 32)
		if err != nil {
			s.Fatal("Failed to parse score: ", err)
		}

		if pv != nil {
			// Prepend unit to test name to maintain backwards compatibility with existing data.
			perfValueName := fmt.Sprintf("%s_%s", unit, testName)
			pv.Set(perf.Metric{
				Name:      perfValueName,
				Variant:   perfValueName,
				Unit:      unit,
				Direction: perf.BiggerIsBetter,
			}, testRating)

			// TODO(oka): Original test additionally exports the same metric with another name
			// like "link_1.8GHz_4GB" (search for get_board_with_frequency_and_memory in graphics_GLBench.py).
			// Confirm if its used and if so port it.
		}

		errMsg := ""
		// Classify result image.
		switch {
		case testRating == -1.0:
			// Test generated GL Error.
			glError := strings.Split(imageFile, "=")[1]
			errMsg = fmt.Sprintf("GLError %s during test", glError)
			failedTests[testName] = "GLError"
		case testRating == 0.0:
			// Tests for which glbench does not generate a meaningful perf score.
			errMsg = "no score for test"
		case strings.Contains(fixedBadImageNames, imageFile):
			// We know the image looked bad at some point in time but we thought
			// it was fixed. Throw an exception as a reminder.
			errMsg = fmt.Sprintf("fixedbad [%s]", imageFile)
			failedTests[testName] = imageFile
		case strings.Contains(knownBadImageNames, imageFile):
			// We have triaged the failure and have filed a tracking bug.
			// Don't throw an exception and remind there is a problem.
			errMsg = fmt.Sprintf("knownbad [%s]", imageFile)
			// This failure is whitelisted so don't add to failedTests.
		case strings.Contains(referenceImageNames, imageFile):
			// Known good reference images (default).
		case imageFile == "none":
			// Tests that do not write images can't fail because of them.
		case noChecksumTest(testName):
			// TODO(ihf): these really should not write any images
		default:
			// Completely unknown images. Report a failure.
			errMsg = fmt.Sprintf("unknown [%s]", imageFile)
			failedTests[testName] = imageFile
		}

		if errMsg != "" {
			fmt.Fprintf(f, "# %s: %s\n", testName, errMsg)
		}
	}

	if len(failedTests) > 0 {
		s.Logf("Some images don't match their reference in %s; please verify that the output images are correct and if so copy them to the reference directory", referenceImageFile)
		s.Errorf("Some images don't match their references: %q; check summary.txt for details", failedTests)
	}
}

// noChecksumTests are tests that do not draw anything.
// They can only be used to check performance.
var noChecksumTests = []string{
	"compositing_no_fill",
	"pixel_read",
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

func reportTemperatureCritical(ctx context.Context, pv *perf.Values, name string) error {
	temp, err := temperatureCritical(ctx)
	if err != nil {
		return errors.Wrap(err, "report temperature critical")
	}
	pv.Set(perf.Metric{
		Name:      name,
		Unit:      "Celsius",
		Direction: perf.SmallerIsBetter,
	}, temp)
	return nil
}

func reportTemperature(pv *perf.Values, name string) error {
	temp, err := temperatureInputMax()
	if err != nil {
		return errors.Wrap(err, "report temperature")
	}
	pv.Set(perf.Metric{
		Name:      name,
		Unit:      "Celsius",
		Direction: perf.SmallerIsBetter,
	}, temp)
	return nil
}

// temperatureInputMax returns the maximum currently observed temperature in Celsius.
func temperatureInputMax() (float64, error) {
	// The files contain temperature input value in millidegree Celsius.
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	const pattern = "/sys/class/hwmon/hwmon*/temp*_input"
	fs, err := filepath.Glob(pattern)
	if err != nil {
		return 0, errors.Wrap(err, "could not get input temperature")
	}
	if len(fs) == 0 {
		return 0, errors.Errorf("could not get input temperature: no file matches %s", pattern)
	}

	res := math.Inf(-1)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, errors.Wrap(err, "could not get input temperature")
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return 0, errors.Wrapf(err, "could not parse %s to get input temperature", f)
		}
		res = math.Max(res, c/1000)
	}
	return res, nil
}

// temperatureCritical returns temperature at which we will see some throttling in the system in Celcius.
func temperatureCritical(ctx context.Context) (float64, error) {
	// The files contain temperature critical max value in millidegree Celsius.
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	const pattern = "/sys/class/hwmon/hwmon*/temp*_crit"
	fs, err := filepath.Glob(pattern)
	if err != nil {
		return 0, errors.Wrap(err, "could not get critical temperature")
	}
	if len(fs) == 0 {
		return 0, errors.Errorf("could not get critical temperature: no file matches %s", pattern)
	}

	// Compute the minimum value among all.
	res := math.Inf(0)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, errors.Wrap(err, "get critical temperature")
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return 0, errors.Wrapf(err, "parse %s to get critical temperature", f)
		}
		// Files can show 0 on certain boards. crbug.com/360249
		if c == 0 {
			continue
		}
		res = math.Min(res, c/1000)
	}
	if 60 <= res && res <= 150 {
		// Normal path.
		return res, nil
	}
	// Got suspicious result; use typical value for the machine.

	var typical float64
	u, err := sysutil.Uname()
	if err != nil {
		return 0, err
	}
	// Today typical for Intel is 98'C to 105'C while ARM is 85'C. Clamp to 98
	// if Intel device or the lowest known value otherwise. crbug.com/360249
	if strings.Contains(u.Machine, "x86") {
		typical = 98
	} else {
		typical = 85
	}
	testing.ContextLogf(ctx, "Computed critical temperature %.1fC is suspicious; returning %.1fC", res, typical)
	return typical, nil
}
