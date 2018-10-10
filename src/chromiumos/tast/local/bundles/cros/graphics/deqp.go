// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    DEQP,
		Desc:    "Runs a pre-CQ-suitable subset of the drawElements Quality Program test suite shipped with test images",
		Attr:    []string{"disabled", "informational"},
		Timeout: time.Duration(len(deqpTests)) * deqpCmdTimeout,
	})
}

// outcomeMap represents the result of running multiple DEQP tests. Each key is
// an outcome (e.g., "pass", "skipped", etc.). The value is a list of DEQP test
// names which resulted in that outcome.
type outcomeMap map[string][]string

// Deadline for the command that runs the DEQP test to complete. According to
// the initialization of graphics_dEQP in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py, this should
// be larger than twice the DEQP watchdog timeout at 30s.
const deqpCmdTimeout = 70 * time.Second

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
	"dEQP-VK.api.info.instance.physical_devices",
	"dEQP-VK.api.info.instance.layers",
	"dEQP-VK.api.info.instance.extensions",
	"dEQP-VK.api.info.device.features",
	"dEQP-VK.api.info.device.queue_family_properties",
	"dEQP-VK.api.info.device.memory_properties",
	"dEQP-VK.api.info.device.layers",
}

// runSingleTest runs a single DEQP test:
//
// - test: name of the DEQP test to run, e.g., "dEQP-GLES2.info.vendor".
//
// - apis: list of supported graphics APIs.
//
// - env: list of environment variables used when running the DEQP test. An
//   element in this list looks like "SHELL=/bin/bash".
//
// - logPath: folder used to store the detailed DEQP log. A log file with name
//   <test case name>.log will be saved in this folder.
//
// This function returns the outcome of the test determined by the result of
// parsing the DEQP log file. If an irrecoverable parsing error occurs,
// "parsefailed" is returned. If the test is not run due to an unsupported
// graphics API, "skipped" is returned.
//
// This function is based on multiple places:
//
// - Initialization of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
// - The _run_tests_individually() method of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
// - The _translate_name_to_api() method of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
// - The _can_run() method of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
// - The _get_executable() method of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
func runSingleTest(ctx context.Context, s *testing.State, test string, apis []graphics.APIType, env []string, logPath string) string {
	// Detect the graphics API to use based on the test name prefix. The map was
	// ported from DEQP_MODULES in
	// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
	// TODO(andrescj): consider putting this in utils.go.
	var deqpPrefixToAPIMap map[string]graphics.APIType = map[string]graphics.APIType{
		"dEQP-EGL":    graphics.EGL,
		"dEQP-GLES2":  graphics.GLES2,
		"dEQP-GLES3":  graphics.GLES3,
		"dEQP-GLES31": graphics.GLES31,
		"dEQP-VK":     graphics.VK,
	}
	api, ok := deqpPrefixToAPIMap[strings.Split(test, ".")[0]]
	if !ok {
		s.Fatalf("Found test with invalid name: %q", test)
	}

	// Check if the detected API is supported.
	supported := false
	for _, a := range apis {
		if api == a {
			supported = true
			break
		}
	}

	if !supported {
		// TODO(andrescj): add the GPU type to this log message.
		s.Logf("Skipping %q due to unsupported API %q", test, api)
		return "skipped"
	}

	// Get the path to the DEQP binary to run for the test.
	p, err := graphics.DEQPExecutable(api)
	if err != nil {
		s.Fatalf("Could not get the executable for %q: %v", api, err)
	}

	// We must be in the executable's directory when running it so that it can
	// find its test data files.
	if err := os.Chdir(filepath.Dir(p)); err != nil {
		s.Fatal("Could not chdir to the executable's folder: ", err)
	}

	// Prepare the command. Note that --deqp-surface-type is either "fbo" or
	// "pbuffer". The latter avoids DEQP assumptions. The --deqp-surface-width
	// and --deqp-surface-height should be the smallest for which all tests
	// run/pass.
	logFile := filepath.Join(logPath, test+".log")
	cmdCtx, cancel := context.WithTimeout(ctx, deqpCmdTimeout)
	defer cancel()
	cmd := testexec.CommandContext(cmdCtx, p,
		"--deqp-case="+test,
		"--deqp-surface-type=pbuffer",
		"--deqp-gl-config-name=rgba8888d24s8ms0",
		"--deqp-log-images=disable",
		"--deqp-watchdog=enable",
		"--deqp-surface-width=256",
		"--deqp-surface-height=256",
		"--deqp-log-filename="+logFile)
	cmd.Env = env
	s.Log("Command: ", cmd.Args)

	// Run the test. Note that we don't care about the exit status code of the
	// command. For example, even if the DEQP test fails, the command can return
	// 0. We base our determination of the outcome entirely on the parsing of
	// the detailed log file.
	cmd.Run()

	if cmdCtx.Err() == context.DeadlineExceeded {
		// If deqp.CmdTimeout is set appropriately, we shouldn't reach this
		// point because the DEQP watchdog should take care of timeouts per
		// test. We make this a fatal error so that we know to revisit our
		// assumptions about the timeout.
		s.Fatal("Command timeout! Please check the deqp.CmdTimeout setting.")
	}

	if stats, nonFailed, err := graphics.ParseDEQPOutput(logFile); err == nil {
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

	// An irrecoverable error occurred during parsing. An irrecoverable error
	// implies that the log file wasn't processed fully. Since we ran a single
	// test, we can attribute the error to that test. Hence, we'll count this
	// error under the "parsefailed" outcome.
	s.Errorf("Irrecoverable parsing error for %v: %v", logFile, err)
	return "parsefailed"
}

// logOutcomeMap outputs a summary of all the tests grouped by outcome. The
// outcomes are sorted in ascending order and output in uppercase.
func logOutcomeMap(s *testing.State, outcomes outcomeMap) {
	// Sort the keys in the map first since the iteration order is not
	// guaranteed: see https://blog.golang.org/go-maps-in-action.
	sortedOutcomes := make([]string, 0, len(outcomes))
	for o := range outcomes {
		sortedOutcomes = append(sortedOutcomes, o)
	}
	sort.Strings(sortedOutcomes)
	s.Log(strings.Repeat("=", 50))
	s.Log("SUMMARY:")
	s.Log(strings.Repeat("=", 50))
	for _, o := range sortedOutcomes {
		s.Log(strings.ToUpper(o), ":")
		for _, t := range outcomes[o] {
			s.Log("  ", t)
		}
	}
	s.Log(strings.Repeat("=", 50))
}

func DEQP(ctx context.Context, s *testing.State) {
	// Start of setup code - this is a port from multiple places:
	//
	// - Initialization of GraphicsApiHelper in
	//   autotest/files/client/cros/graphics/graphics_utils.py.
	//
	// - Initialization of graphics_dEQP in
	//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
	//
	// - The run_once() method of graphics_dEQP in
	//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.

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
	s.Logf("Vulkan support: %v", hasVulkan)

	apis := graphics.SupportedAPIs(glMajor, glMinor, hasVulkan)
	s.Logf("Supported APIs: %v", apis)

	// TODO(andrescj): also extract/log the following in the configuration per
	// graphics_dEQP initialization: board, CPU type, and GPU type. Right now,
	// the board and CPU type seem to be used only for logging. The GPU type is
	// used to deduce test expectations (tests that we expect to pass/fail
	// depending on the GPU).

	// Step 2: get the environment for the DEQP binaries.
	env := graphics.DEQPEnvironment(os.Environ())
	s.Logf("Using environment: %q", env)

	// Step 3: create a location for storing detailed logs.
	logPath := filepath.Join(s.OutDir(), "dEQP-results")
	if err := os.Mkdir(logPath, 0700); err != nil {
		s.Fatalf("Could not create %v: %v", logPath, err)
	}

	// TODO(andrescj): stop services per graphics_dEQP initialization - ui and
	// powerd. Restore after tests are done.

	// End of setup code

	// Step 4: get the list of tests to execute and run them. This is based on
	// the _run_tests_individually() method of graphics_dEQP in
	// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
	outcomes := make(outcomeMap)
	for i, t := range deqpTests {
		s.Logf("[%d/%d] Test: %s", i+1, len(deqpTests), t)
		o := runSingleTest(ctx, s, t, apis, env, logPath)
		s.Log("Result: ", strings.ToUpper(o))
		outcomes[o] = append(outcomes[o], t)
	}
	logOutcomeMap(s, outcomes)
}
