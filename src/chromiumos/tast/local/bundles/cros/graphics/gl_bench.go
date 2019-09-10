// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"context"
	"fmt"
	"os"
	"path/filepath"

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
		Attr:         []string{"informational"},
	})
}

// FIXME(oka): Do proper error handling.
func GLBench(ctx context.Context, s *testing.State) {
	const glbenchDirecotry = "/usr/local/glbench/"

	// Good images.
	referenceImageFile := filepath.Join(glbenchDirecotry, "files/glbench_reference_images.txt")
	// Images that are bad but for which the bug has not been fixed yet.
	knownBadImagesFile := filepath.Join(glbenchDirecotry, "files/glbench_knownbad_images.txt")
	// Images that are bad and for which a fix has been submitted.
	fixedBadImagesFile := filepath.Join(glbenchDirecotry, "files/glbench_fixedbad_images.txt")

	// Run the test, saving is optional and helps with debugging
	// and reference image management. If unknown images are
	// encountered one can take them from the outdir and copy
	// them (after verification) into the reference image dir.
	exefile := filepath.Join(glbenchDirecotry, "bin/glbench")

	// These tests do not draw anything, they can only be used to check
	// performance.
	noChecksumTests := []string{
		"compositing_no_fill",
		"pixel_read",
		"texture_reuse_luminance_teximage2d",
		"texture_reuse_luminance_texsubimage2d",
		"texture_reuse_rgba_teximage2d",
		"texture_reuse_rgba_texsubimage2d",
		"context_glsimple",
		"swap_glsimple",
	}

	unitHigherIsBetter := map[string]bool {
		"mbytes_sec": true,
			"mpixels_sec": true,
			"mtexel_sec": true,
			"mtri_sec": true,
			"mvtx_sec": true,
			"us": false,
			"1280x768_fps": true,
	}

/*
   def initialize(self):
     super(graphics_GLBench, self).initialize()
     # If UI is running, we must stop it and restore later.
     self._services = service_stopper.ServiceStopper(['ui'])
     self._services.stop_services()
 */
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal(err)
	}
	defer func(){
		if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()

	// FIXME: parameterized?
	hasty := false
	var options []string

	options = append(options, "-save", "-outdir=" + s.OutDir())
	if hasty {
		options = append(options, "-hasty")
	}

	cmd := testexec.CommandContext(ctx, exefile, options...)

	var summary string
	if hasty {
		// On BVT the test will not monitor thermals so we will not verify its
		// correct status using PerfControl
		summary = cmd.Output(testexec.DumpLogOnError)
	} else {
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
	fmt.Fprintf(f, `# ---------------------------------------------------
# [%s]
%s

# -------------------------------------------------
# [graphics_GLBench.py postprocessing]
`, cmd, summary)

	   // Analyze the output. Sample:
	   // # board_id: NVIDIA Corporation - Quadro FX 380/PCI/SSE2
	   // # Running: ../glbench -save -outdir=img
	   // swap_swap = 221.36 us [swap_swap.pixmd5-20dbc...f9c700d2f.png]

	   /*
	   results = summary.splitlines()
	   if not results:
	     f.close()
	     raise error.TestFail('Failed: No output from test. Check /tmp/' +
	                          'test_that_latest/graphics_GLBench/summary.txt' +
	                          ' for details.')

	   # The good images, the silenced and the zombie/recurring failures.
	   reference_imagenames = self.load_imagenames(self.reference_images_file)
	   knownbad_imagenames = self.load_imagenames(self.knownbad_images_file)
	   fixedbad_imagenames = self.load_imagenames(self.fixedbad_images_file)

	   # Check if we saw GLBench end as expected (without crashing).
	   test_ended_normal = False
	   for line in results:
	     if line.strip().startswith('@TEST_END'):
	       test_ended_normal = True

	   # Analyze individual test results in summary.
	   # TODO(pwang): Raise TestFail if an error is detected during glbench.
	   keyvals = {}
	   failed_tests = {}
	   for line in results:
	     if not line.strip().startswith('@RESULT: '):
	       continue
	     keyval, remainder = line[9:].split('[')
	     key, val = keyval.split('=')
	     testname = key.strip()
	     score, unit = val.split()
	     testrating = float(score)
	     imagefile = remainder.split(']')[0]

	     if not hasty:
	       higher = self.unit_higher_is_better.get(unit)
	       if higher is None:
	         raise error.TestFail('Failed: Unknown test unit "%s" for %s' %
	                              (unit, testname))
	       # Prepend unit to test name to maintain backwards compatibility with
	       # existing per data.
	       perf_value_name = '%s_%s' % (unit, testname)
	       self.output_perf_value(
	           description=perf_value_name,
	           value=testrating,
	           units=unit,
	           higher_is_better=higher,
	           graph=perf_value_name)
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

////// bin/utils.py

//     Returns temperature at which we will see some throttling in the system.
func temperatureCritical() {
	// FIXME
	//min_temperature = 1000.0
	//paths = _get_hwmon_paths('temp*_crit')
	//for path in paths:
	//temperature = _get_float_from_file(path, 0, None, None) * 0.001
	//# Today typical for Intel is 98'C to 105'C while ARM is 85'C. Clamp to 98
	//# if Intel device or the lowest known value otherwise.
	//	result = utils.system_output('crossystem arch', retain_output=True,
	//	ignore_status=True)
	//if (min_temperature < 60.0) or min_temperature > 150.0:
	//if 'x86' in result:
	//min_temperature = 98.0
	//else:
	//min_temperature = 85.0
	//logging.warning('Critical temperature was reset to %.1fC.',
	//min_temperature)
	//
	//min_temperature = min(temperature, min_temperature)
	//return min_temperature
}