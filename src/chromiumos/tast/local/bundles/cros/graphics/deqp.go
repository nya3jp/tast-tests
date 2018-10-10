// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// The list of DEQP tests to run. Some may be skipped depending on the supported
// graphics APIs. This list is directly obtained from
// autotest/files/client/site_tests/graphics_dEQP/master/bvt.txt.
var deqpTests = [...]string{
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

// Some constant configuration shared by all DEQP tests. Ported from multiple
// places:
//
// - Initialization of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
//
// - The _run_tests_individually() method of graphics_dEQP in
//   autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
const (
	// Surface details. The surface is either "fbo" or "pbuffer". The latter
	// avoids DEQP assumptions. The width and height should be the smallest for
	// which all tests run/pass. These values are used for the
	// --deqp-surface-type, --deqp-surface-width, and --deqp-surface-height
	// flags respectively.
	deqpSurfaceType   = "pbuffer"
	deqpSurfaceWidth  = 256
	deqpSurfaceHeight = 256

	// Name of the GL configuration used for the --deqp-gl-config-name flag.
	deqpGLConfig = "rgba8888d24s8ms0"

	// Used for the --deqp-log-images flag (logging of images).
	deqpLogImages = false

	// Whether to enable the DEQP watchdog (value of the --deqp-watchdog flag).
	deqpWatchdog = true

	// Deadline for the command that runs the DEQP test to complete. According
	// to the initialization of graphics_dEQP, this should be larger than twice
	// the DEQP watchdog timeout at 30s.
	cmdTimeout = 70 * time.Second
)

// deqpPrefixToAPIMap maps the prefix of a DEQP test name to a graphics API
// identifier. Ported from DEQP_MODULES in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
var deqpPrefixToAPIMap map[string]graphics.APIType = map[string]graphics.APIType{
	"dEQP-EGL":    graphics.EGL,
	"dEQP-GLES2":  graphics.GLES2,
	"dEQP-GLES3":  graphics.GLES3,
	"dEQP-GLES31": graphics.GLES31,
	"dEQP-VK":     graphics.VK,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:    DEQP,
		Desc:    "Runs a pre-CQ-suitable subset of the drawElements Quality Program test suite shipped with test images",
		Attr:    []string{"disabled", "informational"},
		Timeout: time.Duration(len(deqpTests)) * cmdTimeout,
	})
}

// runTestsIndividually runs each DEQP test in |tests| separately from the
// others. |apis| is the list of supported graphics APIs. |env| is the list of
// environment variables used for executing the DEQP binaries. |logPath| is the
// folder used to store detailed DEQP logs: each test case will produce a log
// file with name <test case name>.log. This is a port from multiple places:
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
func runTestsIndividually(ctx context.Context, s *testing.State, tests []string, apis []graphics.APIType, env []string, logPath string) map[string][]string {
	outcomes := make(map[string][]string)
	for i, t := range tests {
		s.Logf("[%d/%d] Test: %s", i+1, len(tests), t)

		// Detect the graphics API to use based on the test name prefix.
		api, ok := deqpPrefixToAPIMap[strings.Split(t, ".")[0]]
		if !ok {
			s.Fatalf("Skipping test with invalid name: %q", t)
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
			s.Logf("Skipping %q due to unsupported API %q", t, api)
			outcomes["skipped"] = append(outcomes["skipped"], t)
			continue
		}

		// Get the path to the DEQP binary to run for the test.
		p, err := graphics.DEQPExecutable(api)
		if err != nil {
			s.Fatalf("Could not get the executable for %q: %v", api, err)
		}

		// We must be in the executable's directory when running it so that it
		// can find its test data files.
		if err := os.Chdir(filepath.Dir(p)); err != nil {
			s.Fatalf("Could not chdir to the executable's folder: %v", err)
		}

		// Run the test.
		logFile := filepath.Join(logPath, t+".log")
		logImages := "disable"
		if deqpLogImages {
			logImages = "enable"
		}
		watchdog := "disable"
		if deqpWatchdog {
			watchdog = "enable"
		}
		cmtCtx, _ := context.WithTimeout(ctx, cmdTimeout)
		cmd := testexec.CommandContext(cmtCtx, p,
			"--deqp-case="+t,
			"--deqp-surface-type="+deqpSurfaceType,
			"--deqp-gl-config-name="+deqpGLConfig,
			"--deqp-log-images="+logImages,
			"--deqp-watchdog="+watchdog,
			"--deqp-surface-width="+strconv.Itoa(deqpSurfaceWidth),
			"--deqp-surface-height="+strconv.Itoa(deqpSurfaceHeight),
			"--deqp-log-filename="+logFile)
		cmd.Env = env
		s.Logf("Command: %v", cmd.Args)
		cmd.Run()

		if cmtCtx.Err() == context.DeadlineExceeded {
			// If |cmdTimeout| is set appropriately, we shouldn't reach this
			// point because the DEQP watchdog should take care of timeouts per
			// test. We make this a fatal error so that we know to revisit our
			// assumptions about the timeout.
			s.Fatalf("COMMAND TIMEOUT!")
		}

		if stats, nonFailed, err := graphics.ParseDEQPOutput(logFile); err == nil {
			// Do some sanity checks on the parsing results.
			if len(stats) != 1 {
				s.Fatalf("Unexpected parsing result for %v: len(stats) = %v", logFile, len(stats))
			}
			if len(nonFailed) > 1 {
				s.Fatalf("Unexpected parsing result for %v: len(nonFailed) = %v", logFile, len(nonFailed))
			}
			var result string
			for r, c := range stats {
				if c != 1 {
					s.Fatalf("Unexpected parsing result for %v: stats[%v] = %v", logFile, r, c)
				}
				result = r
			}
			outcomes[result] = append(outcomes[result], t)
			if len(nonFailed) == 1 {
				s.Logf("Result: %v", strings.ToUpper(result))
			} else {
				s.Errorf("Result: %v", strings.ToUpper(result))
			}
		} else {
			// An irrecoverable error occurred during parsing. An irrecoverable
			// error means that the log file wasn't processed fully. Since we
			// ran a single test, we can attribute the error to that test.
			// Hence, we'll count this error under the "parsefailed" outcome.
			s.Errorf("Irrecoverable parsing error for %v: %v", logFile, err)
			outcomes["parsefailed"] = append(outcomes["parsefailed"], t)
		}
	}
	return outcomes
}

func prettySummary(outcomes map[string][]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "================================================================================\n")
	fmt.Fprintf(&b, "|                                   SUMMARY                                    |\n")
	fmt.Fprintf(&b, "================================================================================\n")
	for outcome, tests := range outcomes {
		fmt.Fprintf(&b, "| %-77s|\n", strings.ToUpper(outcome))
		for _, t := range tests {
			fmt.Fprintf(&b, "|   %-75s|\n", t)
		}
	}
	fmt.Fprintf(&b, "================================================================================\n")
	return b.String()
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
	if err := os.RemoveAll(logPath); err != nil {
		s.Fatal("Could not remove the log location: ", err)
	}
	if err := os.Mkdir(logPath, 0700); err != nil {
		s.Fatal("Could not create the log location: ", err)
	}
	s.Logf("Location for detailed logs: %v", logPath)

	// TODO(andrescj): stop services per graphics_dEQP initialization - ui and
	// powerd. Restore after tests are done.

	// End of setup code

	// Step 4: get the list of tests to execute and run them.
	outcomes := runTestsIndividually(ctx, s, deqpTests[:], apis, env, logPath)
	s.Logf("\n%s", prettySummary(outcomes))
}
