// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/inputoverlay"
	"chromiumos/tast/local/chrome/uiauto"
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
	inputoverlay.SetupTestApp(ctx, s, func(params inputoverlay.TestParams) error {
		// Start up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard")
		}
		defer kb.Close()

		if err := uiauto.Combine("Tap overlay keys and ensure proper behavior",
			// Execute keystrokes corresponding to tap buttons.
			inputoverlay.TapOverlayButton(kb, "m", &params, inputoverlay.TopTap),
			inputoverlay.TapOverlayButton(kb, "n", &params, inputoverlay.BotTap),
			// Execute keystrokes corresponding to hold-release controls.
			action.Sleep(inputoverlay.WaitForActiveInputTime),
			inputoverlay.MoveOverlayButton(kb, "w", &params),
			action.Sleep(inputoverlay.WaitForActiveInputTime),
			inputoverlay.MoveOverlayButton(kb, "a", &params),
			action.Sleep(inputoverlay.WaitForActiveInputTime),
			inputoverlay.MoveOverlayButton(kb, "s", &params),
			action.Sleep(inputoverlay.WaitForActiveInputTime),
			inputoverlay.MoveOverlayButton(kb, "d", &params),
		)(ctx); err != nil {
			return errors.Wrap(err, "one or more keystrokes failed")
		}

		return nil
	})
}
