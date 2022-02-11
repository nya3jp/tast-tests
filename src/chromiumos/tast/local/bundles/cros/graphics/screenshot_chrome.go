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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenshotChrome,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Takes a screenshot using Chrome",
		Contacts:     []string{"jkardatzke@chromium.org"},
		Attr:         []string{"group:graphics", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
	})
}

func ScreenshotChrome(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	err := sshot.SShot(ctx, s, cr, func(ctx context.Context, path string) error {
		return screenshot.CaptureChrome(ctx, cr, path)
	})
	if err != nil {
		s.Fatal("Failure in screenshot comparison: ", err)
	}
}
