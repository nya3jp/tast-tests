// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/emojipicker"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmoji,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that right click input field and select emoji with physical keyboard",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}, {
			Name:      "informational",
			ExtraAttr: []string{"informational"},
			// Skip on grunt & zork boards due to b/213400835.
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("grunt", "zork")),
		}}})
}

func PhysicalKeyboardEmoji(ctx context.Context, s *testing.State) {
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

	s.Run(ctx, "recently_used", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_emoji_input")

		action := uiauto.Combine("validate recentl used emojis",
			its.TriggerEmojiPickerFromContextMenu(inputField),
			ui.WaitUntilExists(emojipicker.RecentUsedMenu),
			kb.AccelAction("ESC"),
			emojipicker.WaitUntilGone(tconn),
		)
		if err := useractions.NewUserAction(
			"Frequently used emoji is updated",
			action,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeInputField:   string(inputField),
					useractions.AttributeTestScenario: "validate recent submitted emoji is updated",
				},
				Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
			}).Run(ctx); err != nil {
			s.Fatal("Failed to dismiss emoji picker by tapping ESC key: ", err)
		}
	})

	s.Run(ctx, "dismiss_by_esc", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_dismiss_by_esc")
		scenario := "Dismiss emoji picker by tapping ESC key"
		action := uiauto.Combine(scenario,
			its.Clear(inputField),
			its.TriggerEmojiPickerFromContextMenu(inputField),
			kb.AccelAction("ESC"),
			emojipicker.WaitUntilGone(tconn),
		)
		if err := useractions.NewUserAction(
			scenario,
			action,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{useractions.AttributeInputField: string(inputField)},
				Tags:       []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
			}).Run(ctx); err != nil {
			s.Fatal("Failed to dismiss emoji picker by tapping ESC key: ", err)
		}
	})
}
