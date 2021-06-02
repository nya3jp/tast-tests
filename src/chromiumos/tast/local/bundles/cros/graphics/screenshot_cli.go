// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/graphics/sshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ScreenshotCLI,
		Desc:     "Takes a screenshot using the CLI",
		Contacts: []string{"nya@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		// screenshot command broken on RK3399: https://issuetracker.google.com/issues/172219229
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.SkipOnPlatform("gru")),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
	})
}

func ScreenshotCLI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	err := sshot.SShot(ctx, s, cr, func(ctx context.Context, path string) error {
		return screenshot.Capture(ctx, path)
	})
	if err != nil {
		s.Fatal("Failure in screenshot comparison: ", err)
	}
}
