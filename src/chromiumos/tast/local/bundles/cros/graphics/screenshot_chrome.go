// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/graphics/sshot"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenshotChrome,
		Desc:         "Takes a screenshot using Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ScreenshotChrome(ctx context.Context, s *testing.State) {
	err := sshot.SShot(ctx, s, screenshot.CaptureChrome)
	if err != nil {
		s.Fatal("Failure in screenshot comparison: ", err)
	}
}
