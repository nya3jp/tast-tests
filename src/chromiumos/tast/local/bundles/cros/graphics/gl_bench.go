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
	"strconv"
	"strings"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Porting from http://cs/chromeos_public/src/third_party/autotest/files/client/site_tests/graphics_GLBench/graphics_GLBench.py?l=15&rcl=c0090b2ed9e137c6094effd004989d2d22699797
// FIXME(oka): remove this.

func init() {
	testing.AddTest(&testing.Test{
		Func: GLBench,
		Desc: "Run glbench, a benchmark that times graphics intensive activities",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"oka@chromium.org", // Tast port.
		},
		Attr: []string{"informational"},
	})
}	

const glbenchDir = "/usr/local/glbench/"

var (
	// referenceImageFile contains good images.
	referenceImageFile = filepath.Join(glbenchDir, "files/glbench_reference_images.txt")
	// knownBadImagesFile contains images that are bad but for which the bug has not been fixed yet.
	knownBadImagesFile = filepath.Join(glbenchDir, "files/glbench_knownbad_images.txt")
	// Images that are bad and for which a fix has been submitted.
	fixedBadImagesFile = filepath.Join(glbenchDir, "files/glbench_fixedbad_images.txt")

	// glbench is the executable for performance testing.
	glbench = filepath.Join(glbenchDir, "bin/glbench")

	// noChecksumTests are tests do not draw anything.
	// They can only be used to check performance.
	noChecksumTests = []string{
		"compositing_no_fill",
		"pixel_read",
		"texture_reuse_luminance_teximage2d",
		"texture_reuse_luminance_texsubimage2d",
		"texture_reuse_rgba_teximage2d",
		"texture_reuse_rgba_texsubimage2d",
		"context_glsimple",
		"swap_glsimple",
	}

	unitHigherIsBetter = map[string]bool{
		"mbytes_sec":   true,
		"mpixels_sec":  true,
		"mtexel_sec":   true,
		"mtri_sec":     true,
		"mvtx_sec":     true,
		"us":           false,
		"1280x768_fps": true,
	}
)

// GLBench runs glbench and reports its performance.
// TODO(oka): Port the portion corresponding to hasty = false from graphics_GLBench.py.
func GLBench(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal(err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	
	args := []string{"-save", "-outdir="+s.OutDir()}
	// Run the test, saving is optional and helps with debugging
	// and reference image management. If unknown images are
	// encountered one can take them from the outdir and copy
	// them (after verification) into the reference image dir.
	cmd := testexec.CommandContext(ctx, glbench, args...)

	var summary string
	if hasty {
		// On BVT the test will not monitor thermals so we will not verify its
		// correct status using PerfControl
		summary = cmd.Output(testexec.DumpLogOnError)
	} else {
		if err := func() error {
			tempCrit, err := temperatureCritical(ctx)
			if err != nil {
				return err
			}
			pv := perf.NewValues()
			pv.Append(perf.Metric{
				Name:      "temperature_critical",
				Unit:      "Celsius",
				Direction: perf.SmallerIsBetter,
			}, tempCrit)

			temp, err := temperatureInputMax()
			if err != nil {
				return err
			}
			pv.Append(perf.Metric{
				Name:      "temperature_1_start",
				Unit:      "Celsius",
				Direction: perf.SmallerIsBetter,
			}, temp)

			if err := pv.Save(s.OutDir()); err != nil {
				return err
			}
			return nil
		}(); err != nil {

		}

		return nil
		// FIXME
		/*
			utils.report_temperature_critical(self, 'temperature_critical')
			utils.report_temperature(self, 'temperature_1_start')
			# Wrap the test run inside of a PerfControl instance to make machine
			# behavior more consistent.
				with perf.PerfControl() as pc:
			if not pc.verify_is_valid():
			raise error.TestFail('Failed: %s' % pc.get_error_reason())
			utils.report_temperature(self, 'temperature_2_before_test')

			# Run the test. If it gets the CPU too hot pc should notice.
				summary = utils.run(cmd,
				stderr_is_expected=False,
				stdout_tee=utils.TEE_TO_LOGS,
				stderr_tee=utils.TEE_TO_LOGS).stdout
			if not pc.verify_is_valid():
			# Defer error handling until after perf report.
				pc_error_reason = pc.get_error_reason()
		*/
	}
	// FIXME

	// Write a copy of stdout to help debug failures.
	resultPath := filepath.Join(s.OutDir(), "summary.txt")
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, `# ---------------------------------------------------
# [%s]
%s

# -------------------------------------------------
# [graphics_GLBench.py postprocessing]
`, cmd, summary)

	results := strings.Split(summary, "\n")
	if len(results) == 0 {
		s.Fatal("hogehoge")
	}
	// Analyze the output. Sample:
	// # board_id: NVIDIA Corporation - Quadro FX 380/PCI/SSE2
	// # Running: ../glbench -save -outdir=img
	// swap_swap = 221.36 us [swap_swap.pixmd5-20dbc...f9c700d2f.png]

	must := func(x string, err error) string {
		if err != nil {
			s.Fatal(err)
		}
		return x
	}
	// The good images, the silenced and the zombie/recurring failures.
	referenceImageNames := must(loadImageNames(referenceImageFile))
	knownBadImageNames := must(loadImageNames(knownBadImagesFile))
	fixedBadImageNames := must(loadImageNames(fixedBadImagesFile))

	// Check if we saw GLBench end as expected (without crashing).
	testEndedNormal := false
	for _, line := range results {
		if strings.HasPrefix(strings.TrimSpace(line), "@TEST_END") {
			testEndedNormal = true
		}
	}

	// Analyze individual test results in summary.
	// TODO(pwang): Raise TestFail if an error is detected during glbench.
	keyVals := make(map[string]string)
	var failedTests []string

	for _, line := range results {
		if !strings.HasPrefix(strings.TrimSpace(line), "@RESULT: ") {
			continue
		}
		keyval, remainder := strings.Split(line[9:], "[")
		kv := strings.Split(keyval, "=")
		key, val := kv[0], kv[1]
		testName := strings.TrimSpace(key)
		su := strings.Split(val, " ")
		score, unit := su[0], su[1]
		testRating, err := strconv.ParseFloat(score, 32)
		if err != nil {
			s.Fatal()
		}
		imageFile := strings.Split(remainder, "]")[0]

		if !hasty {
			higher, ok := unitHigherIsBetter[unit]
			if !ok {
				s.Fatal()
			}
			dir := perf.SmallerIsBetter
			if higher {
				dir = perf.BiggerIsBetter
			}
			// Prepend unit to test name to maintain backwards compatibility with
			// existing per data.
			perfValueName := fmt.Sprintf("%s_%s", unit, testName)
			pv := perf.NewValues()
			pv.Set(perf.Metric{
				Name:      perfValueName,
				Unit:      unit,
				Direction: dir,
			}, testRating)

			// Add extra value to the graph distinguishing different boards.
		}

		/*
			   if not hasty:

				 # Add extra value to the graph distinguishing different boards.
				 variant = utils.get_board_with_frequency_and_memory()
				 desc = '%s-%s' % (perf_value_name, variant)
				 self.output_perf_value(
					 description=desc,
					 value=testrating,
					 units=unit,
					 higher_is_better=higher,
					 graph=perf_value_name)

			   # Classify result image.
			   if testrating == -1.0:
				 # Tests that generate GL Errors.
				 glerror = imagefile.split('=')[1]
				 f.write('# GLError ' + glerror + ' during test (perf set to -3.0)\n')
				 keyvals[testname] = -3.0
				 failed_tests[testname] = 'GLError'
			   elif testrating == 0.0:
				 # Tests for which glbench does not generate a meaningful perf score.
				 f.write('# No score for test\n')
				 keyvals[testname] = 0.0
			   elif imagefile in fixedbad_imagenames:
				 # We know the image looked bad at some point in time but we thought
				 # it was fixed. Throw an exception as a reminder.
				 keyvals[testname] = -2.0
				 f.write('# fixedbad [' + imagefile + '] (setting perf as -2.0)\n')
				 failed_tests[testname] = imagefile
			   elif imagefile in knownbad_imagenames:
				 # We have triaged the failure and have filed a tracking bug.
				 # Don't throw an exception and remind there is a problem.
				 keyvals[testname] = -1.0
				 f.write('# knownbad [' + imagefile + '] (setting perf as -1.0)\n')
				 # This failure is whitelisted so don't add to failed_tests.
			   elif imagefile in reference_imagenames:
				 # Known good reference images (default).
				 keyvals[testname] = testrating
			   elif imagefile == 'none':
				 # Tests that do not write images can't fail because of them.
				 keyvals[testname] = testrating
			   elif self.is_no_checksum_test(testname):
				 # TODO(ihf): these really should not write any images
				 keyvals[testname] = testrating
			   else:
				 # Completely unknown images. Raise a failure.
				 keyvals[testname] = -2.0
				 failed_tests[testname] = imagefile
				 f.write('# unknown [' + imagefile + '] (setting perf as -2.0)\n')


		*/
	}

	/*
	   f.close()
	   if not hasty:
	     utils.report_temperature(self, 'temperature_3_after_test')
	     self.write_perf_keyval(keyvals)

	   # Raise exception if images don't match.
	   if failed_tests:
	     logging.info('Some images are not matching their reference in %s.',
	                  self.reference_images_file)
	     logging.info('Please verify that the output images are correct '
	                  'and if so copy them to the reference directory.')
	     raise error.TestFail('Failed: Some images are not matching their '
	                          'references. Check /tmp/'
	                          'test_that_latest/graphics_GLBench/summary.txt'
	                          ' for details.')

	   if not test_ended_normal:
	     raise error.TestFail(
	         'Failed: No end marker. Presumed crash/missing images.')
	   if pc_error_reason:
	     raise error.TestFail('Failed: %s' % pc_error_reason)
	*/
}

func loadImageNames(filename string) (string, error) {
	b, err := ioutil.ReadFile(filename)
	return string(b), err

	/*
		"""Loads text file with MD5 file names.

		@param filename: name of file to load.
		imagenames = os.path.join(self.autodir, filename)
		with open(imagenames, 'r') as f:
		imagenames = f.read()
		return imagenames
	*/
}

// Returns the maximum currently observed temperature.
func temperatureInputMax() (float64, error) {
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	fs, err := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	if err != nil {
		return 0, err
	}

	res := math.Inf(-1)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, err
		}
		c, err := strconv.ParseFloat(string(b), 64)
		if err != nil {
			return 0, err
		}
		res = math.Max(res, c/1000)
	}
	return res, nil
}

func perfTest(ctx context.Context, f func()) error {
	// Wait
	init, err := temperatureInputMax()
	if err != nil {
		return nil, err
	}
	critical, err := temperatureCritical(ctx)
	if err != nil {
		return nil, err
	}
}

func waitForIdleCPU(ctx context.Context, utilization: float64) {
	getCPUUsage()
}

// Sets the kernel governor mode to the highest setting.
// Returns previous governor state.
func setHighPerformanceMode() error {
	fs, err := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor")
	if err != nil {
		return err
	}
	original := getScalingGovernorStates()
	set_scaling_governors
	// FIXME.
}

// Returns temperature at which we will see some throttling in the system in celcius.
func temperatureCritical(ctx context.Context) (float64, error) {
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	fs, err := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_crit")
	if err != nil {
		return 0, err
	}

	// Compute the minimum value among all.
	res := math.Inf(0)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, err
		}
		c, err := strconv.ParseFloat(string(b), 64)
		if err != nil {
			return 0, err
		}
		// Files can show 0 on certain boards. crbug.com/360249
		if c == 0 {
			continue
		}
		res = math.Min(res, c/1000)
	}
	if 60 <= res && res <= 150 {
		return res, nil
	}

	var typical float64
	u, err := sysutil.Uname()
	if err != nil {
		return 0, err
	}
	// Today typical for Intel is 98'C to 105'C while ARM is 85'C. Clamp to 98
	// if Intel device or the lowest known value otherwise. crbug.com/360249
	// TODO(oka): Consider removing this workaround checking the log.
	if strings.Contains(u.Machine, "x86") {
		typical = 98
	} else {
		typical = 85
	}
	testing.ContextLogf(ctx, "Computed critical temperature %.1fC is suspicious; returning %.1fC.", res, typical)
	return typical, nil
}
