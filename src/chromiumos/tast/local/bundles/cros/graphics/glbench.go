// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type envType int
type glbenchConfig struct {
	hasty       bool    // hasty indicates to run glbench in hasty mode.
	environment envType // environment indicates the environment type glbench is running on.
}

const (
	envCros envType = iota
	envDebian
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GLBench,
		Desc: "Run glbench (a benchmark that times graphics intensive activities), check results and report its performance",
		Contacts: []string{
			"andrescj@chromium.org",
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
			"oka@chromium.org", // Tast port
		},
		SoftwareDeps: []string{"no_qemu"},
		Params: []testing.Param{
			{
				Name:      "",
				Val:       glbenchConfig{environment: envCros},
				Timeout:   3 * time.Hour,
				ExtraAttr: []string{"group:graphics", "graphics_nightly"},
			}, {
				Name:      "hasty",
				Val:       glbenchConfig{hasty: true, environment: envCros},
				ExtraAttr: []string{"group:mainline"},
				Timeout:   5 * time.Minute,
			}, {
				Name:              "crostini",
				Pre:               crostini.StartedGPUEnabledBuster(),
				Val:               glbenchConfig{environment: envDebian},
				ExtraAttr:         []string{"group:graphics", "graphics_weekly"},
				ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
				Timeout:           60 * time.Minute,
			}, {
				Name:              "crostini_hasty",
				Pre:               crostini.StartedGPUEnabledBuster(),
				Val:               glbenchConfig{hasty: true, environment: envDebian},
				ExtraAttr:         []string{"group:graphics", "graphics_perbuild"},
				ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
				Timeout:           5 * time.Minute,
			}},
	})
}

const glbenchDir = "/usr/local/glbench/"

var (
	resultsOutDir = "glbench_results"

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
	testConfig := s.Param().(glbenchConfig)

	var pv *perf.Values // nil when hasty == true
	if !testConfig.hasty {
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

		must(reportTemperature(ctx, pv, "temperature_1_start"))

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
		must(reportTemperature(ctx, pv, "temperature_2_before_test"))
	}

	// Run the test, saving is optional and helps with debugging
	// and reference image management. If unknown images are
	// encountered one can take them from the outdir and copy
	// them (after verification) into the reference image dir.
	args := []string{"-save", "-notemp"}
	if testConfig.hasty {
		args = append(args, "-hasty")
	}
	var cmd *testexec.Cmd
	switch testConfig.environment {
	case envCros:
		// If UI is running, we must stop it and restore later.
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			s.Fatal("Failed on set up: ", err)
		}
		defer func() {
			if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}
		}()
		// glbench is the executable for performance testing.
		glbench := filepath.Join(glbenchDir, "bin/glbench")
		args = append(args, "-outdir="+filepath.Join(s.OutDir(), resultsOutDir))
		cmd = testexec.CommandContext(ctx, glbench, args...)
	case envDebian:
		// Disable the display to avoid vsync.
		if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
			s.Fatal("Failed to disable the display: ", err)
		}
		defer func() {
			if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}
		}()
		cont := s.PreValue().(crostini.PreData).Container
		if err := cont.Command(ctx, "dpkg", "-s", "glbench").Run(); err != nil {
			s.Fatal("Failed checking for glbench in dpkg -s: ", err)
		}
		// In crostini, glbench is preinstalled in PATH.
		args = append(args, "-outdir="+resultsOutDir)
		cmd = cont.Command(ctx, append([]string{"glbench"}, args...)...)
	default:
		s.Fatal("Failed to recognize envType: ", testConfig.environment)
	}

	cmdLine := shutil.EscapeSlice(cmd.Args)
	// On BVT the test will not monitor thermals so we will not verify its
	// correct status using PerfControl.
	s.Log("Running ", cmdLine)
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run test: ", err)
	}
	summary := string(b)

	// In Debian environment, we need to copy result files out of the container.
	if testConfig.environment == envDebian {
		cont := s.PreValue().(crostini.PreData).Container
		if err := cont.GetFile(ctx, resultsOutDir, s.OutDir()); err != nil {
			s.Fatal("Cannot get the results from container: ", err)
		}
	}
	if pv != nil {
		if err := reportTemperature(ctx, pv, "temperature_3_after_test"); err != nil {
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
	var failedTests []string
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
			// Confirm if it's used and if so port it.
		}

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
			failedTests = append(failedTests, testName)
		}

		if errMsg != "" {
			fmt.Fprintf(f, "# %s: %s\n", testName, errMsg)
		}
	}

	if len(failedTests) > 0 {
		sort.Strings(failedTests)
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

func reportTemperature(ctx context.Context, pv *perf.Values, name string) error {
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
