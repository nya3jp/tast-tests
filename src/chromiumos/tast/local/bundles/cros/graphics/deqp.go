// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

// deqpTests contains the names of the DEQP tests to run. Some may be skipped
// depending on the supported graphics APIs. This list is directly obtained from
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

func init() {
	testing.AddTest(&testing.Test{
		Func: DEQP,
		Desc: "Runs a pre-CQ-suitable subset of the drawElements Quality Program test suite shipped with test images",
		Attr: []string{"disabled", "informational"},
	})
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
	s.Logf("Location for detailed logs: %v", logPath)

	// TODO(andrescj): stop services per graphics_dEQP initialization - ui and
	// powerd. Restore after tests are done.

	// End of setup code

	// TODO(andrescj): run the DEQP tests.
}
