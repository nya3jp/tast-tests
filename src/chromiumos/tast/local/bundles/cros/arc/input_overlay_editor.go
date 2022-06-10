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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputOverlayEditor,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test for gaming input overlay key mapping editor correctness",
		Contacts:     []string{"pjlee@google.com", "cuicuiruan@google.com", "arc-app-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"input-overlay-menu.png"},
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

func InputOverlayEditor(ctx context.Context, s *testing.State) {
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
		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)

		// CUJ: Attempts to change binding to illegal keys.
		s.Log("Editor CUJ #1: key mappings changed to illegal keys")
		if err := uiauto.Combine("mappings changed to illegal keys",
			// Close educational dialog.
			ui.LeftClick(nodewith.Name("Got it").HasClass("LabelButtonLabel")),
			// Open game controls.
			ui.LeftClick(nodewith.Name("Game controls").HasClass("ImageButton")),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "w" to "ESC" (NOTE: "w" key is used because, unlike the
			// "n" key, the associated on-screen error messages have no overlapping text,
			// and thus it has the highest chance of success with text detection).
			ui.LeftClick(nodewith.Name("w").HasClass("LabelButtonLabel")),
			kb.TypeKeyAction(input.KEY_ESC),
			// Verify illegal mapping.
			waitForMultiple(uda, "Unsupported", "ported", "pported"),
			// Change mapping of "w" to "w".
			kb.TypeAction("w"),
			// Verify illegal mapping.
			waitForMultiple(uda, "Same", "ame"),
			// Change mapping of "w" to "CTRL"
			kb.TypeKeyAction(input.KEY_LEFTCTRL),
			// Verify illegal mapping.
			waitForMultiple(uda, "Unsupported", "ported", "pported"),
			// Close out.
			uda.Tap(uidetection.Word("Cancel")),
		)(ctx); err != nil {
			s.Error("Failed to verify illegal keys: ", err)
			// Reset activity.
			if err := gio.CloseAndRelaunchActivity(ctx, &params); err != nil {
				s.Fatal("Failed to reset application after failed CUJ: ", err)
			}
		}

		// CUJ: Change key mappings and then press cancel.
		s.Log("Editor CUJ #2: key mappings changes canceled")
		if err := uiauto.Combine("cancel changed mapping",
			// Open game controls.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "n" to "l".
			ui.LeftClick(nodewith.Name("n").HasClass("LabelButtonLabel")),
			kb.TypeAction("l"),
			uda.Tap(uidetection.Word("Cancel")),
			// Verify old mapping still exists.
			ui.WaitUntilExists(nodewith.Name("n").HasClass("LabelButtonLabel")),
		)(ctx); err != nil {
			s.Error("Failed to verify canceled mapping: ", err)
			// Reset activity.
			if err := gio.CloseAndRelaunchActivity(ctx, &params); err != nil {
				s.Fatal("Failed to reset application after failed CUJ: ", err)
			}
		}

		// CUJ: Key of key binding changed to another existing key bind.
		s.Log("Editor CUJ #3: key mapping changed to a non-existing key bind")
		if err := uiauto.Combine("mapping unbound",
			// Open game controls.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "n" to "m"
			ui.LeftClick(nodewith.Name("w").HasClass("LabelButtonLabel")),
			kb.TypeAction("g"),
			// Save binding.
			uda.Tap(uidetection.Word("Save")),
			// Verify original "m" binding doesn't exist anymore (i.e. the current "m"
			// binding taps at the bottom tap button, not the top tap button).
			gio.MoveOverlayButton(kb, "g", &params),
		)(ctx); err != nil {
			s.Fatal("Failed to verify unbound mapping: ", err)
		}

		// CUJ: Key of key binding changed to another existing key bind.
		s.Log("Editor CUJ #4: key mapping changed to another existing key bind")
		if err := uiauto.Combine("mapping unbound",
			// Open game controls.
			uda.Tap(uidetection.CustomIcon(s.DataPath("input-overlay-menu.png"))),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "n" to "m"
			ui.LeftClick(nodewith.Name("n").HasClass("LabelButtonLabel")),
			kb.TypeAction("m"),
			// Save binding.
			uda.Tap(uidetection.Word("Save")),
			// Verify original "m" binding doesn't exist anymore (i.e. the current "m"
			// binding taps at the bottom tap button, not the top tap button).
			gio.TapOverlayButton(kb, "m", &params, gio.BotTap),
		)(ctx); err != nil {
			s.Fatal("Failed to verify unbound mapping: ", err)
		}

		// CUJ: Close and reopen test application after changing key bindings.
		s.Log("Editor CUJ #5: close and reopen application, after changing key mappings")
		if err := uiauto.Combine("mapping unbound",
			// Close and reopen test application.
			closeAndReopen(&params),
			// Verify "m" binding still taps at bottom tap button.
			gio.TapOverlayButton(kb, "m", &params, gio.BotTap),
		)(ctx); err != nil {
			s.Error("Failed to verify mappings saved after app closure and reopening: ", err)
		}

		return nil
	})
}

// waitForMultiple returns true if any of the listed words are found via ACUITI.
func waitForMultiple(uda *uidetection.Context, words ...string) action.Action {
	return func(ctx context.Context) error {
		for _, word := range words {
			if err := uda.WaitUntilExists(uidetection.Word(word))(ctx); err != nil {
				continue
			}
			return nil
		}
		return errors.New("no listed words found")
	}
}

// closeAndReopen returns a function that closes the current test application activity and relaunches it.
// It is also important to reassign the "Activity" parameter of the given TestParams
// pointer, since the "SetupTestApp" function called initially defers the closing
// of the original instance of the application.
func closeAndReopen(params *gio.TestParams) action.Action {
	return func(ctx context.Context) error {
		err := gio.CloseAndRelaunchActivity(ctx, params)
		if err != nil {
			return errors.Wrap(err, "failed to create a new ArcInputOverlayTest activity")
		}
		return nil
	}
}
