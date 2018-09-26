// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/graphics/sshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenshotCLI,
		Desc:         "Takes a screenshot using the CLI",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "screenshot"},
	})
}

func ScreenshotCLI(s *testing.State) {
	err := sshot.SShot(s, func(ctx context.Context, cr *chrome.Chrome, path string) error {
		cmd := testexec.CommandContext(ctx, "screenshot", "--internal", path)
		if err := cmd.Run(); err != nil {
			// We do not abort here because:
			// - screenshot command might have failed just because the internal display is not on yet
			// - Context deadline might be reached while taking a screenshot, which should be
			//   reported as "Screenshot does not contain expected pixels" rather than
			//   "screenshot command failed".
			cmd.DumpLog(ctx)
			return err
		}
		return nil
	})
	if err != nil {
		s.Fatal("Failure in screenshot comparison: ", err)
	}
}
