// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/emojipicker"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
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
		Func:         PhysicalKeyboardEmoji,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that right click input field and select emoji with physical keyboard",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kefka")),
			},
			{
				Name:      "informational",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				// Skip on grunt & zork boards due to b/213400835.
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("grunt", "zork")),
			},
			/* Disabled due to <1% pass rate over 30 days. See b/246818430
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kefka")),
			}
			*/
		},
	})
}

func PhysicalKeyboardEmoji(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	inputField := testserver.TextAreaInputField
	inputEmoji := "😂"
	ui := emojipicker.NewUICtx(tconn)

	s.Run(ctx, "emoji_input", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_emoji_input")
		if err := its.InputEmojiWithEmojiPicker(uc, inputField, inputEmoji)(ctx); err != nil {
			s.Fatal("Failed to verify emoji picker: ", err)
		}
	})

	// Tap ESC key to dismiss emoji picker.
	// This test is also covered in browser test https://source.chromium.org/chromium/chromium/src/+/main:chrome/browser/ui/views/bubble/bubble_contents_wrapper_unittest.cc;drc=7059ce9510b276afe73ce0bc389a72b58f482420;l=154.
	// Keep this test since it is still required to complete the entire E2E test journey.
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
				useractions.AttributeFeature:      useractions.FeatureEmojiPicker,
			},
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
				useractions.AttributeFeature:      useractions.FeatureEmojiPicker,
			},
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
					Attributes: map[string]string{
						useractions.AttributeFeature: useractions.FeatureEmojiPicker,
					},
				},
			),
		)(ctx); err != nil {
			s.Fatal("Failed to clear recently used emoji: ", err)
		}
	})
}
