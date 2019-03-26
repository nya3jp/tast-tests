// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/graphics/drm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DRM,
		Desc: "Verifies DRM-related test binaries run successfully",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"display_backlight"},
	})
}

func DRM(ctx context.Context, s *testing.State) {
	if err := drm.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer drm.TearDown(ctx)

	const timeout = 20 * time.Second
	drm.RunTest(ctx, s, timeout, "/usr/local/bin/drm_cursor_test")
	drm.RunTest(ctx, s, timeout, "/usr/local/bin/linear_bo_test")
	drm.RunTest(ctx, s, timeout, "/usr/local/bin/null_platform_test")
	drm.RunTest(ctx, s, timeout, "/usr/local/bin/vgem_test")
}
