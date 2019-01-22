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
		Func:         ScreenshotCLI,
		Desc:         "Takes a screenshot using the CLI",
		Contacts:     []string{"nya@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "screenshot"},
		Pre:          chrome.LoggedIn(),
	})
}

func ScreenshotCLI(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	err := sshot.SShot(ctx, s, cr, func(ctx context.Context, path string) error {
		return screenshot.Capture(ctx, path)
	})
	if err != nil {
		s.Fatal("Failure in screenshot comparison: ", err)
	}
}
