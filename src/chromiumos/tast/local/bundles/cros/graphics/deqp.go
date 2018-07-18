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
