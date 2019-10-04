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
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GLBench,
		Desc: "Run glbench, a benchmark that times graphics intensive activities",
		Contacts: []string{
			"andrescj@chromium.org",
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
			"oka@chromium.org", // Tast port.
		},
		Attr: []string{"informational"},
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

// GLBench runs glbench and reports its performance.
// TODO(oka): Port the portion corresponding to hasty = false from graphics_GLBench.py.
func GLBench(ctx context.Context, s *testing.State) {
	// If UI is running, we must stop it and restore later.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed on set up: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()

	args := []string{"-save", "-outdir=" + s.OutDir(), "-hasty"}
	// Run the test, saving is optional and helps with debugging
	// and reference image management. If unknown images are
	// encountered one can take them from the outdir and copy
	// them (after verification) into the reference image dir.
	cmd := testexec.CommandContext(ctx, glbench, args...)
	cmdLine := shutil.EscapeSlice(cmd.Args)

	// On BVT the test will not monitor thermals so we will not verify its
	// correct status using PerfControl.
	s.Log("Running ", cmdLine)
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run test: ", err)
	}
	summary := string(b)

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

		// Third value (unit) is currently unused. TODO(oka): use it in hasty = false.
		testName, score, _, imageFile := m[1], m[2], m[3], m[4]
		testRating, err := strconv.ParseFloat(score, 32)
		if err != nil {
			s.Fatal("Failed to parse score: ", err)
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
