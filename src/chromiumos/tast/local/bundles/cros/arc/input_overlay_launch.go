// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/inputoverlay"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputOverlayLaunch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Installs the GIO test application and checks for launch correctness",
		Contacts:     []string{"pjlee@google.com", "cuicuiruan@google.com", "arc-app-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithInputOverlay",
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
		Timeout: 20 * time.Minute,
	})
}

func InputOverlayLaunch(ctx context.Context, s *testing.State) {
	inputoverlay.SetupTestApp(ctx, s, func(params inputoverlay.TestParams) error {
		// Start up UIAutomator.
		ui := uiauto.New(params.TestConn)

		if err := uiauto.Combine("Find gaming input overlay UI elements",
			// Find input overlay game control.
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.ClassName("ImageButton")),
			// Find input overlay tap buttons.
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("m")),
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("n")),
			// Find input overlay joystick buttons.
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("w")),
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("d")),
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("s")),
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("a")),
		)(ctx); err != nil {
			return errors.Wrap(err, "one or more items not loaded")
		}

		return nil
	})
}
