// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/gio"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputOverlayDisplay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test for gaming input overlay menu correctness",
		Contacts:     []string{"pjlee@google.com", "cuicuiruan@google.com", "arc-app-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"input-overlay-menu-close.png", "input-overlay-menu-switch.png", "input-overlay-menu.png"},
		Fixture:      "arcBootedWithInputOverlay",
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

func InputOverlayDisplay(ctx context.Context, s *testing.State) {
	gio.SetupTestApp(ctx, s, func(params gio.TestParams) error {
		// Start up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard")
		}
		defer kb.Close()
		// Start up UIAutomator.
		ui := uiauto.New(params.TestConn).WithTimeout(time.Minute)
		// Start up ACUITI.
		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithScreenshotStrategy(uidetection.ImmediateScreenshot).WithTimeout(time.Minute)
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, params.TestConn)

		// CUJ: Hide game overlay.
		s.Log("Display CUJ #1: hide game overlay")
		if err := uiauto.Combine("hide game overlay",
			// Close educational dialog.
			ui.LeftClick(nodewith.Name("Got it").HasClass("LabelButtonLabel")),
			// Open game controls.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			// Tap bottom menu switch.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu-switch.png")).Below(uidetection.Word("Customize"))),
			// Exit out of menu.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu-close.png")).Below(uidetection.Word("BUTTON"))),
			// Poll UI elements no longer exist, but overlay is still responsive.
			ui.Gone(nodewith.Name("m").HasClass("LabelButtonLabel")),
			gio.TapOverlayButton(kb, "m", &params, gio.TopTap),
			ui.Gone(nodewith.Name("w").HasClass("LabelButtonLabel")),
			gio.MoveOverlayButton(kb, "w", &params),
			// Poll edits can still be done.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			uda.Tap(uidetection.Word("Customize")),
			ui.WaitUntilExists(nodewith.Name("m").HasClass("LabelButtonLabel")),
			ui.WaitUntilExists(nodewith.Name("w").HasClass("LabelButtonLabel")),
			// Exit out.
			uda.Tap(uidetection.Word("Cancel")),
		)(ctx); err != nil {
			s.Error("Failed to verify game overlay hidden: ", err)
			// Reset activity.
			s.Fatal("I want a tree")
			if err := gio.CloseAndRelaunchActivity(ctx, &params); err != nil {
				s.Fatal("Failed to reset application after failed CUJ: ", err)
			}
		}

		// CUJ: Disable game overlay.
		s.Log("Display CUJ #2: disable game overlay")
		if err := uiauto.Combine("disable game overlay",
			// Open game controls.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			// Tap top menu switch.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu-switch.png")).Above(uidetection.Word("Customize"))),
			// Exit out of menu.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu-close.png")).Below(uidetection.Word("BUTTON"))),
			// Poll UI elements no longer exist, and overlay is unresponsive.
			ui.Gone(nodewith.Name("m").HasClass("LabelButtonLabel")),
			not(gio.TapOverlayButton(kb, "m", &params, gio.TopTap)),
			ui.Gone(nodewith.Name("w").HasClass("LabelButtonLabel")),
			not(gio.MoveOverlayButton(kb, "w", &params)),
			// Check "Customize" button disabled.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			uda.Tap(uidetection.Word("Customize")),
			not(uda.Gone(uidetection.Word("Customize"))),
		)(ctx); err != nil {
			s.Error("Failed to verify game overlay disabled: ", err)
			s.Fatal("I want a tree")
		}

		return nil
	})
}

// not returns a function that returns an error if the given action did not return
// an error, and returns nil if the given action resulted in an error.
func not(a action.Action) action.Action {
	return func(ctx context.Context) error {
		if err := a(ctx); err == nil {
			return errors.Wrap(err, "action succeeded unexpectedly")
		}
		return nil
	}
}
