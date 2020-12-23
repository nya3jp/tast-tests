// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// Output may be prepended by other chars, and order of elements is not defined.
// Examples:
// org.chromium.arc.test.app... INSTRUMENTATION_STATUS: gl_version=OpenGL ES 3.1
// INSTRUMENTATION_STATUS: gl_extensions=GL_EXT_DEPTH_CLAMP GL_EXT_texture_query_lod
var apkOutputRegex = regexp.MustCompile(`(?m)` + // Enable multiline
	`^.*INSTRUMENTATION_STATUS: (.*)=(.*)$`)

// Looking for something like:
// OK (1 test)
var apkOKRegex = regexp.MustCompile(`\nOK \(\d+ tests?\)\n*$`)
var astcRegex = regexp.MustCompile(`GL_KHR_texture_compression_astc`)
var glesVersionRegex = regexp.MustCompile(`OpenGL ES\s+(\d+).(\d+)`)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GLESMinRequirements,
		Desc:         "Checks whether the OpenGL ES minimun requirements are satisfied",
		Contacts:     []string{"ricardo@chromium.org", "arc-gaming+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func GLESMinRequirements(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC

	if err := a.Install(ctx, arc.APKPath("ArcGamePerformanceTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Launch test via "am instrument" since it is easier to capture the output.
	const className = "GLESMinReqTest"
	cmd := a.Command(ctx, "am", "instrument", "-w", "-e", "class", "org.chromium.arc.testapp.gameperformance."+className, "org.chromium.arc.testapp.gameperformance")
	bytes, err := cmd.Output()
	out := string(bytes)
	if err != nil {
		s.Fatal("Failed to execute APK: ", err)
	}

	// Make sure test is completed successfully.
	if !apkOKRegex.MatchString(out) {
		s.Log("Test output: ", out)
		s.Fatal("Test did not completed successfully")
	}

	results := make(map[string]string)
	for _, m := range apkOutputRegex.FindAllStringSubmatch(out, -1) {
		results[m[1]] = m[2]
	}

	// Check GLES version.
	glVersion, ok := results["gl_version"]
	if !ok {
		s.Log("Test output: ", out)
		s.Fatal("Failed to find 'gl_vesion'")
	}
	matches := glesVersionRegex.FindStringSubmatch(glVersion)
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		s.Log("OpenGL ES version: ", glVersion)
		s.Fatal("Failed to parse OpenGL ES version: ", err)
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		s.Log("OpenGL ES version: ", glVersion)
		s.Fatal("Failed to parse OpenGL ES version: ", err)
	}

	if major < 3 || (major == 3 && minor < 1) {
		s.Log("GLES version: ", glVersion)
		s.Fatalf("Unexpected GLES version, got %d.%d, want: >= 3.1", major, minor)
	}

	// Check ETC1 support.
	supportsETC1, ok := results["supports_ETC1"]
	if !ok {
		s.Fatal("Failed to find 'supports_ETC1'")
	}
	if supportsETC1 != "true" {
		s.Log("Test output: ", out)
		s.Fatalf("Unexpected ETC1 value, got: %q, want='true'", supportsETC1)
	}

	// No need to check for ETC2 compressed texture format.
	// The ETC2/EAC texture compression formats are guaranteed to be available when using the OpenGL ES 3.0 API.
	// From: https://developer.android.com/guide/topics/graphics/opengl

	// ASTC support is already included in GLES 3.2+.
	supportsASTC := (major > 3) || (major == 3 && minor >= 2)

	// Check for ASTC LDR support. There are 3 profiles:
	// a) GL_KHR_texture_compression_astc_ldr
	// b) GL_KHR_texture_compression_astc_hdr (includes a)
	// c) GL_KHR_texture_compression_astc (includes b)
	// The test passes it at least a) is supported.
	if !supportsASTC && !astcRegex.MatchString(results["gl_extensions"]) {
		s.Log("Test output: ", out)
		s.Fatal("ASTC LDR texture format not supported")
	}
}
