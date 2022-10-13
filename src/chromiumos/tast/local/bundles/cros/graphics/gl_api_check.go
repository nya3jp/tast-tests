// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GLAPICheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the version of GLES",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Timeout:      5 * time.Minute,
	})
}

// GLAPICheck verifies that the GLES api is the correct version
func GLAPICheck(ctx context.Context, s *testing.State) {
	// query the supported graphics APIs.
	glMajor, glMinor, err := graphics.GLESVersion(ctx)
	if err != nil {
		s.Fatal("Could not obtain the OpenGL version: ", err)
	}
	s.Logf("Found gles%d.%d", glMajor, glMinor)

	// Make sure the GLES version >= 1.3
	if glMajor > 1 && glMinor >= 3 || glMajor >= 2 {
		s.Log("warning : Please add missing extension check.Details crbug.com/413079")
		return
	}
	s.Fatal("missing EGL/GLES version info")
}
