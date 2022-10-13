// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GLAPICheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the version of GLES",
		// TODO(syedfaaiz): Add to CQ once it is green and stable.
		Attr: []string{"group:graphics", "graphics_nightly"},
		Contacts: []string{"syedfaaiz@google.com",
			"chromeos-gfx@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Timeout:      1 * time.Minute,
	})
}

var (
	// Regexp to extract the first occurrences of a floating number
	eglInfoRe = regexp.MustCompile(`EGL version string: ([0-9]+).([0-9]+)`)
)

func extractEGLVersion(s *testing.State, matched [][]string) (major,
	minor int, err error) {
	eglVerMajor, err := strconv.Atoi(matched[0][1])
	if err != nil {
		return 0, 0, errors.Errorf("could not convert extracted egl major regex value: %s , to int", matched[0][1])
	}
	eglVerMinor, err := strconv.Atoi(matched[0][2])
	if err != nil {
		return 0, 0, errors.Errorf("could not convert extracted egl minor regex value: %s , to int", matched[0][2])
	}
	return eglVerMajor, eglVerMinor, nil
}

// GLAPICheck verifies that the GLES api is the correct version
func GLAPICheck(ctx context.Context, s *testing.State) {
	// query the supported graphics APIs.
	glMajor, glMinor, err := graphics.GLESVersion(ctx)
	if err != nil {
		s.Fatal("Could not obtain the OpenGL version: ", err)
	}
	s.Logf("Found GLES%d.%d", glMajor, glMinor)
	// Make sure the GLES version >= 3.0
	if glMajor <= 3 && glMinor < 1 {
		s.Fatal("GLES version is older than 3.0")
	}
	//TODO(syedfaaiz): GPU user space drivers MUST support EGL 1.3
	// verify the our devices are 1.3 and above and support the following
	// extensions:
	//EGL_EXT_image_dma_buf_import, EGL_EXT_image_dma_buf_import_modifiers,
	//EGL_KHR_surfaceless_context, EGL_KHR_fence_sync, OES_EGL_image_external

	stdout, _, err := testexec.CommandContext(ctx, "eglinfo").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("eglinfo command failed to run: ", err)
	}
	// Use regexp to find first occurrence of version for EGL.
	matched := eglInfoRe.FindAllStringSubmatch(string(stdout), -1)
	if len(matched) == 0 {
		s.Fatal("regex could not detect eglinfo version: ", err)
	}
	eglMajor, eglMinor, err := extractEGLVersion(s, matched)
	if err != nil {
		s.Fatal("An error occured while extracting egl version data: ", err)
	}
	s.Logf("Found EGL%v.%v", eglMajor, eglMinor)
	//Make sure the EGL version >= 1.3
	if eglMajor <= 1 && eglMinor < 3 {
		s.Fatalf("eglversion 1.0 or greater required, current version %s.%s", eglMajor, eglMinor)
	}
	return
}
