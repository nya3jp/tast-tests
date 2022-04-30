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
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputOverlayTouchInjector,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test for gaming input overlay touch injector correctness",
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

func InputOverlayTouchInjector(ctx context.Context, s *testing.State) {
	testutil.SetupTestApp(ctx, s, func(params testutil.TestParams) error {
		// Start up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard")
		}
		defer kb.Close()
		// Start up UIAutomator.
		ui := uiauto.New(params.TestConn).WithTimeout(time.Minute)
		// Obtain window surface bounds.
		loc, err := params.Activity.SurfaceBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to obtain activity window bounds")
		}
		appWidth := loc.BottomRight().X - loc.TopLeft().X
		appHeight := loc.BottomRight().Y - loc.TopLeft().Y
		params.WindowContentSize = coords.NewPoint(appWidth, appHeight)

		if err := uiauto.Combine("Tap overlay keys and ensure proper behavior",
			// Close educational dialog.
			ui.LeftClick(nodewith.Name("Got it").HasClass("LabelButtonLabel")),
			// Execute keystrokes corresponding to tap buttons.
			testutil.TapOverlayButton(kb, "m", &params, testutil.TopTap),
			testutil.TapOverlayButton(kb, "n", &params, testutil.BotTap),
			// Execute keystrokes corresponding to hold-release controls.
			testutil.MoveOverlayButton(kb, "w", &params),
			testutil.MoveOverlayButton(kb, "a", &params),
			testutil.MoveOverlayButton(kb, "s", &params),
			testutil.MoveOverlayButton(kb, "d", &params),
		)(ctx); err != nil {
			return errors.Wrap(err, "one or more keystrokes failed")
		}

		return nil
	})
}
