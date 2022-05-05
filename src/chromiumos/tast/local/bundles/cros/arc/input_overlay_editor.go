// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/testutil"
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

func InputOverlayEditor(ctx context.Context, s *testing.State) {
	testutil.SetupTestApp(ctx, s, func(params testutil.TestParams) error {
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
		if err := uiauto.Combine("Mappings changed to illegal keys",
			// Open game controls.
			ui.LeftClick(nodewith.ClassName("ImageButton")),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "w" to "ESC" (NOTE: "w" key is used because, unlike the
			// "n" key, the associated on-screen error messages have no overlapping text,
			// and thus it has the highest chance of success with text detection).
			ui.LeftClick(nodewith.Name("w")),
			kb.TypeKeyAction(input.KEY_ESC),
			// Verify illegal mapping.
			waitForMultiple(uda, "Unsupported", "ported", "pported"),
			// Change mapping of "w" to "w".
			kb.TypeAction("w"),
			// Verify illegal mapping.
			uda.WaitUntilExists(uidetection.Word("same")),
			// Change mapping of "w" to "CTRL"
			kb.TypeKeyAction(input.KEY_LEFTCTRL),
			// Verify illegal mapping.
			waitForMultiple(uda, "Unsupported", "ported", "pported"),
			// Close out.
			uda.Tap(uidetection.Word("Cancel")),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to verify illegal keys")
		}

		// CUJ: Change key mappings and then press cancel.
		if err := uiauto.Combine("Cancel changed mapping",
			// Open game controls.
			ui.LeftClick(nodewith.ClassName("ImageButton")),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "n" to "l".
			ui.LeftClick(nodewith.Name("n")),
			kb.TypeAction("l"),
			uda.Tap(uidetection.Word("Cancel")),
			// Verify old mapping still exists.
			ui.WaitUntilExists(nodewith.Name("n")),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to verify canceled mapping")
		}

		// CUJ: Key of key binding changed to another existing key bind.
		if err := uiauto.Combine("Mapping unbound",
			// Open game controls.
			ui.LeftClick(nodewith.ClassName("ImageButton")),
			uda.Tap(uidetection.Word("Customize")),
			// Change mapping of "n" to "m"
			ui.LeftClick(nodewith.Name("n")),
			kb.TypeAction("m"),
			// Save binding.
			uda.Tap(uidetection.Word("Save")),
			// Verify original "m" binding doesn't exist anymore (i.e. the current "m"
			// binding taps at the bottom tap button, not the top tap button).
			testutil.TapOverlayButton(kb, "m", &params, testutil.BotTap),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to verify unbound mapping")
		}

		// CUJ: Close and reopen test application after changing key bindings.
		if err := uiauto.Combine("Mapping unbound",
			// Close and reopen test application.
			closeAndReopen(&params),
			// Verify "m" binding still taps at bottom tap button.
			testutil.TapOverlayButton(kb, "m", &params, testutil.BotTap),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to verify mappings saved after app closure and reopening")
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
func closeAndReopen(params *testutil.TestParams) action.Action {
	return func(ctx context.Context) error {
		// Close current test application instance.
		params.Activity.Stop(ctx, params.TestConn)
		// Relaunch another test application instance.
		act, err := testutil.RelaunchActivity(ctx, params)
		if err != nil {
			return errors.Wrap(err, "failed to create a new ArcInputOverlayTest activity")
		}
		// Reassign "Activity" field in params.
		*params.Activity = *act
		return nil
	}
}
