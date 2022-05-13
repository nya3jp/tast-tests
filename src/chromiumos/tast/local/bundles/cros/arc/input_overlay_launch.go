// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/uidetection"
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
		Data: []string{"input-overlay-top-button.png",
			"input-overlay-bottom-button.png",
			"input-overlay-menu.png",
			"input-overlay-joystick.png"},
		Fixture: "arcBootedWithInputOverlay",
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
		Timeout: 5 * time.Minute,
	})
}

func InputOverlayLaunch(ctx context.Context, s *testing.State) {
	testutil.SetupTestApp(ctx, s, func(params testutil.TestParams) error {
		// Start up UIAutomator.
		ui := uiauto.New(params.TestConn).WithTimeout(time.Minute)
		// Start up ACUITI.
		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)

		if err := uiauto.Combine("Find gaming input overlay UI elements",
			// Tap educational dialog.
			ui.LeftClick(nodewith.Name("Got it").HasClass("LabelButtonLabel")),
			// Find input overlay game control.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			// Find input overlay tap buttons.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("input-overlay-top-button.png"))),
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("input-overlay-bottom-button.png"))),
			// Find input overlay joystick buttons.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("input-overlay-joystick.png"))),
		)(ctx); err != nil {
			return errors.Wrap(err, "one or more items not loaded")
		}

		return nil
	})
}
