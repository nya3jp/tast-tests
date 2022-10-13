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
		Timeout:      5 * time.Minute,
	})
}

var (
	// Regexp to extract the first occurrences of a floating number
	eglInfoRe = regexp.MustCompile((`(\d+\.)?(\*|\d+)`))
)

// GLAPICheck verifies that the GLES api is the correct version
func GLAPICheck(ctx context.Context, s *testing.State) {
	// query the supported graphics APIs.
	glMajor, glMinor, err := graphics.GLESVersion(ctx)
	if err != nil {
		s.Fatal("Could not obtain the OpenGL version: ", err)
		return
	}
	s.Logf("Found gles%d.%d", glMajor, glMinor)
	// Make sure the GLES version >= 3.0
	if glMajor < 3 {
		s.Fatal("GLES version is older than 3.0")
	}
	s.Log("warning : Please add missing extension check.Details crbug.com/413079")

	stdout, _, err := testexec.CommandContext(ctx, "eglinfo").SeparatedOutput(testexec.DumpLogOnError)
	// Use regexp to find first occurrence of version for EGL.
	matched := string(eglInfoRe.FindString(string(stdout)))
	if err != nil && len(matched) == 0 {
		s.Fatal("regex could not detect eglinfo version")
		return
	}
	eglVer, err := strconv.ParseFloat(matched, 64)
	if err != nil {
		s.Fatalf("Could not convert extracted regex value: %s , to float", eglVer)
		return
	}
	s.Log("EGL version = ", eglVer)
	if eglVer < 1.0 {
		s.Fatalf("eglversion 1.0 or greater required, current version %s", eglVer)
	}
	return
}
