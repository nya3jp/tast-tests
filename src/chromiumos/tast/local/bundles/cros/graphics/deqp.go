// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DEQP,
		Desc:     "Runs a pre-CQ-suitable subset of the drawElements Quality Program test suite shipped with test images",
		Contacts: []string{"andrescj@chromium.org", "ihf@chromium.org", "chromeos-gfx@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

// deqpTests contains the names of the DEQP tests to run. Some may be skipped
// depending on the supported graphics APIs. This list is directly obtained from
// autotest/files/client/site_tests/graphics_dEQP/master/bvt.txt.
var deqpTests = []string{
	"dEQP-GLES2.info.vendor",
	"dEQP-GLES2.info.renderer",
	"dEQP-GLES2.info.version",
	"dEQP-GLES2.info.shading_language_version",
	"dEQP-GLES2.info.extensions",
	"dEQP-GLES2.info.render_target",
	"dEQP-GLES2.functional.prerequisite.state_reset",
	"dEQP-GLES2.functional.prerequisite.clear_color",
	"dEQP-GLES2.functional.prerequisite.read_pixels",
	"dEQP-GLES3.info.vendor",
	"dEQP-GLES3.info.renderer",
	"dEQP-GLES3.info.version",
	"dEQP-GLES3.info.shading_language_version",
	"dEQP-GLES3.info.extensions",
	"dEQP-GLES3.info.render_target",
	"dEQP-GLES3.functional.prerequisite.state_reset",
	"dEQP-GLES3.functional.prerequisite.clear_color",
	"dEQP-GLES3.functional.prerequisite.read_pixels",
	"dEQP-GLES31.info.vendor",
	"dEQP-GLES31.info.renderer",
	"dEQP-GLES31.info.version",
	"dEQP-GLES31.info.shading_language_version",
	"dEQP-GLES31.info.extensions",
	"dEQP-GLES31.info.render_target",
	"dEQP-VK.info.build",
	"dEQP-VK.info.device",
	"dEQP-VK.info.platform",
	"dEQP-VK.info.memory_limits",
	"dEQP-VK.api.smoke.create_sampler",
	"dEQP-VK.api.smoke.create_shader",
	"dEQP-VK.api.smoke.triangle",
	"dEQP-VK.api.smoke.triangle_ext_structs",
	"dEQP-VK.api.smoke.asm_triangle",
	"dEQP-VK.api.smoke.asm_triangle_no_opname",
	"dEQP-VK.api.smoke.unused_resolve_attachment",
}

// testNameToAPI extracts the graphics API that should be used based on a DEQP
// test name. An error is returned if the name is invalid. This is a port of the
// _translate_name_to_api() method in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
// TODO(andrescj): consider moving this to utils.go and adding tests for it.
func testNameToAPI(test string) (graphics.APIType, error) {
	deqpPrefixToAPIMap := map[string]graphics.APIType{
		"dEQP-EGL":    graphics.EGL,
		"dEQP-GLES2":  graphics.GLES2,
		"dEQP-GLES3":  graphics.GLES3,
		"dEQP-GLES31": graphics.GLES31,
		"dEQP-VK":     graphics.VK,
	}
	if api, ok := deqpPrefixToAPIMap[strings.Split(test, ".")[0]]; ok {
		return api, nil
	}
	return graphics.UnknownAPI, errors.Errorf("%q is not a valid test name", test)
}

// canRunTest returns true iff the DEQP test named test can be run according to
// the supported graphics APIs (passed as apis). If the graphics API cannot be
// inferred from the test name, an error is returned. This function is based on
// the _can_run() method of graphics_dEQP in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
// TODO(andrescj): consider moving this to utils.go and adding tests for it.
func canRunTest(test string, apis []graphics.APIType) (bool, error) {
	testAPI, err := testNameToAPI(test)
	if err != nil {
		return false, err
	}
	for _, a := range apis {
		if testAPI == a {
			return true, nil
		}
	}
	return false, nil
}

// runSingleTest runs a single DEQP test named test, e.g.,
// "dEQP-GLES2.info.vendor" in a child process (which means, e.g., a new
// graphics context for the test). env lists the environment variables to set
// when running the test, e.g., "SHELL=/bin/bash". The test's log is written to
// a file named <test>.log within logDir.
//
// This function returns the outcome of the test determined by the result of
// parsing the test's log file. If an unrecoverable parsing error occurs,
// "parsefailed" is returned.
//
// This function is based on multiple places:
//
//  - Initialization of graphics_dEQP in
//    autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
//  - The _run_tests_individually() method of graphics_dEQP in
//    autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
//  - The _get_executable() method of graphics_dEQP in
//    autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
func runSingleTest(ctx context.Context, s *testing.State, test string, env []string, logDir string) string {
	// Get the path to the DEQP binary to run for the test.
	api, err := testNameToAPI(test)
	if err != nil {
		s.Fatalf("Could not infer the API for %q: %v", test, err)
	}
	p, err := graphics.DEQPExecutable(api)
	if err != nil {
		s.Fatalf("Could not get the executable for %q: %v", api, err)
	}

	// Prepare the command. Note that --deqp-surface-type is either "fbo" or
	// "pbuffer". The latter avoids DEQP assumptions. The --deqp-surface-width
	// and --deqp-surface-height should be the smallest for which all tests
	// run/pass.
	logFile := filepath.Join(logDir, test+".log")
	cmd := testexec.CommandContext(ctx, p,
		"--deqp-case="+test,
		"--deqp-surface-type=pbuffer",
		"--deqp-gl-config-name=rgba8888d24s8ms0",
		"--deqp-log-images=disable",
		"--deqp-watchdog=enable",
		"--deqp-surface-width=256",
		"--deqp-surface-height=256",
		"--deqp-log-filename="+logFile)
	// We should be in the executable's directory when running it so that it can
	// find its test data files.
	cmd.Dir = filepath.Dir(p)
	cmd.Env = env
	s.Log("Command: ", cmd.Args)

	// Run the test. Note that we don't care about the exit status code of the
	// command. For example, even if the DEQP test fails, the command can return
	// 0. We base our determination of the outcome entirely on the parsing of
	// the detailed log file.
	// TODO(andrescj): since cmd.Run() returns an error, maybe we should look
	// into it and fatally fail for unexpected errors not related to dEQP.
	cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		// In the original code, we impose a timeout on the command (on top of
		// the DEQP watchdog). Here, the deadline is imposed by the overall Tast
		// test timeout. That means that if enough DEQP tests time out, we will
		// just stop running the Tast test.
		cmd.DumpLog(ctx)
		s.Fatal("Absolute timeout! Things seem too broken")
	}

	stats, nonFailed, err := graphics.ParseDEQPOutput(logFile)
	if err != nil {
		// An unrecoverable error occurred during parsing. An unrecoverable
		// error implies that the log file wasn't processed fully. Since we ran
		// a single test, we can attribute the error to that test. Hence, we'll
		// count this error under the "parsefailed" outcome.
		s.Logf("Unrecoverable parsing error for %v: %v", logFile, err)
		return "parsefailed"
	}

	// Do some sanity checks on the parsing results.
	if len(stats) != 1 {
		s.Fatalf("Unexpected parsing result for %v: got %v stats; want 1", logFile, len(stats))
	}
	if len(nonFailed) > 1 {
		s.Fatalf("Unexpected parsing result for %v: got %v non-failed tests; want at most 1", logFile, len(nonFailed))
	}
	var outcome string
	for r, c := range stats {
		if c != 1 {
			s.Fatalf("Unexpected parsing result for %v: got %v tests for outcome %v; want 1", logFile, c, r)
		}
		outcome = r
	}
	return outcome
}

func DEQP(ctx context.Context, s *testing.State) {
	// Start of setup code - this is a port from multiple places:
	//
	//  - Initialization of GraphicsApiHelper in
	//    autotest/files/client/cros/graphics/graphics_utils.py.
	//
	//  - Initialization of graphics_dEQP in
	//    autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
	//
	//  - The run_once() method of graphics_dEQP in
	//    autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.

	// TODO(andrescj): port GraphicsTest initialization and clean up from
	// autotest/files/client/cros/graphics/graphics_utils.py to prepare for the
	// test and clean up at the end.

	// Step 1: query the supported graphics APIs.
	glMajor, glMinor, err := graphics.GLESVersion(ctx)
	if err != nil {
		s.Fatal("Could not obtain the OpenGL version: ", err)
	}
	s.Logf("Found gles%d.%d", glMajor, glMinor)

	hasVulkan, err := graphics.SupportsVulkanForDEQP(ctx)
	if err != nil {
		s.Fatal("Could not check for Vulkan support: ", err)
	}
	s.Log("Vulkan support: ", hasVulkan)

	apis := graphics.SupportedAPIs(glMajor, glMinor, hasVulkan)
	s.Log("Supported APIs: ", apis)

	// TODO(andrescj): also extract/log the following in the configuration per
	// graphics_dEQP initialization: board, CPU type, and GPU type. Right now,
	// the board and CPU type seem to be used only for logging. The GPU type is
	// used to deduce test expectations (tests that we expect to pass/fail
	// depending on the GPU).

	// Step 2: get the environment for the DEQP binaries.
	env := graphics.DEQPEnvironment(os.Environ())
	s.Logf("Using environment: %q", env)

	// Step 3: create a location for storing detailed logs.
	logDir := filepath.Join(s.OutDir(), "dEQP-results")
	if err := os.Mkdir(logDir, 0700); err != nil {
		s.Fatalf("Could not create %v: %v", logDir, err)
	}

	// TODO(andrescj): stop services per graphics_dEQP initialization - ui and
	// powerd. Restore after tests are done.

	// End of setup code

	// Step 4: get the list of tests to execute and run them. This is based on
	// the _run_once() and _run_tests_individually() methods of graphics_dEQP in
	// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
	for i, t := range deqpTests {
		s.Logf("[%d/%d] Test: %v", i+1, len(deqpTests), t)

		// Check if the test is supported in the DUT.
		if canRun, err := canRunTest(t, apis); err != nil {
			s.Fatal("Could not check if test is supported: ", err)
		} else if !canRun {
			// TODO(andrescj): add the GPU type to this log message.
			s.Logf("Skipping %q due to unsupported API", t)
			continue
		}

		// Actually run the test.
		if o := runSingleTest(ctx, s, t, env, logDir); graphics.DEQPOutcomeIsFailure(o) {
			s.Errorf("Result for %q: %v", t, strings.ToUpper(o))
		} else {
			s.Logf("Result for %q: %v", t, strings.ToUpper(o))
		}
	}

	// TODO(andrescj): maybe output some counts, like # passes, # failures,
	// #skipped per the run_once() method of graphics_dEQP in
	// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
}
