// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/emojipicker"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		// TODO(b/221789208): Merge test into PhysicalKeyboardEmoji once it is stable.
		Func:         PhysicalKeyboardEmojiInformational,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that right click input field and select emoji with physical keyboard",
		Contacts:     []string{"shengjun@chromium.org", "jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}, {
			Name: "informational",
			// Skip on grunt & zork boards due to b/213400835.
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("grunt", "zork")),
		}}})
}

func PhysicalKeyboardEmojiInformational(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField
	inputEmoji := "😂"
	ui := emojipicker.NewUICtx(tconn)

	s.Run(ctx, "emoji_input", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_emoji_input")
		if err := its.InputEmojiWithEmojiPicker(uc, inputField, inputEmoji).Run(ctx); err != nil {
			s.Fatal("Failed to verify emoji picker: ", err)
		}
	})

	// Tap ESC key to dismiss emoji picker.
	dismissByKeyboardAction := uiauto.UserAction(
		"Dismiss emoji picker",
		uiauto.Combine("dismiss emoji picker by tapping ESC key",
			kb.AccelAction("ESC"),
			emojipicker.WaitUntilGone(tconn),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: "Dismiss emoji picker by tapping ESC key",
			},
			Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
		},
	)

	// Mouse click to dismiss emoji picker.
	dismissByMouseAction := uiauto.UserAction(
		"Dismiss emoji picker",
		uiauto.Combine("dismiss emoji picker by mouse click",
			func(ctx context.Context) error {
				emojiPickerLoc, err := ui.Location(ctx, emojipicker.RootFinder)
				if err != nil {
					return errors.Wrap(err, "failed to get emoji picker location")
				}
				// Click anywhere outside emoji picker will dismiss it.
				// Using TopRight + 50 is safe in this case.
				clickPoint := coords.Point{
					X: emojiPickerLoc.TopRight().X + 50,
					Y: emojiPickerLoc.TopRight().Y,
				}
				return ui.MouseClickAtLocation(0, clickPoint)(ctx)
			},
			emojipicker.WaitUntilGone(tconn),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: "Dismiss emoji picker by mouse click",
			},
			Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
		},
	)

	s.Run(ctx, "recently_used", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_recently")

		if err := uiauto.Combine("validate recently used emojis",
			its.TriggerEmojiPickerFromContextMenu(inputField),
			// Clear recent used emojis.
			uiauto.UserAction(
				"Clear recently used emoji",
				uiauto.Combine("clear recently used emoji",
					ui.LeftClick(emojipicker.RecentUsedMenu),
					ui.LeftClick(emojipicker.ClearRecentlyUsedButtonFinder),
					ui.WaitUntilGone(emojipicker.RecentUsedMenu),
					dismissByKeyboardAction,
					// Launch emoji picker again to confirm it is not only removed from UI.
					its.TriggerEmojiPickerFromContextMenu(inputField),
					ui.WaitUntilGone(emojipicker.RecentUsedMenu),
					dismissByMouseAction,
				),
				uc,
				&useractions.UserActionCfg{
					Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
				},
			),
		)(ctx); err != nil {
			s.Fatal("Failed to clear recently used emoji: ", err)
		}
	})
}
