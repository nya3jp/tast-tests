// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSuggestion,
		Desc:         "Checks emoji suggestions with physical keyboard typing",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshell,
		}, {
			Name:              "guest",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshellInGuest,
		}, {
			Name:              "incognito",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshell,
		}, {
			// Only run informational tests in consumer mode.
			Name:              "informational",
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			Pre:               pre.NonVKClamshell,
		}}})
}

func PhysicalKeyboardEmojiSuggestion(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// PK emoji suggestion only works in English(US).
	inputMethod := ime.EnglishUS

	// Activate function checks the current IME. It does nothing if the given input method is already in-use.
	// It is called here just in case IME has been changed in last test.
	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	its, err := testserver.LaunchInMode(ctx, cr, tconn, strings.HasSuffix(s.TestName(), "incognito"))
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	if strings.HasSuffix(s.TestName(), "incognito") {
		uc.SetAttribute(useractions.AttributeIncognitoMode, strconv.FormatBool(true))
		// Incognito mode only works on a certain web page.
		// It should be reverted once server tear down.
		defer uc.RemoveAttribute(useractions.AttributeIncognitoMode)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	const inputField = testserver.TextInputField

	const (
		word  = "yum"
		emoji = "🤤"
	)

	emojiCandidateWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	emojiCharFinder := nodewith.Name(emoji).Ancestor(emojiCandidateWindowFinder).HasClass("StyledLabel")
	learnMoreFinder := nodewith.Name("Learn more").Ancestor(emojiCandidateWindowFinder).HasClass("ImageButton")
	ui := uiauto.New(tconn)

	validateInputUserAction := func(testScenario string, isEmojiSuggestionEnabled bool) *useractions.UserAction {
		action := uiauto.Combine("validate emoji suggestion",
			its.Clear(inputField),
			its.ClickFieldAndWaitForActive(inputField),
			kb.TypeAction(word),
			kb.AccelAction("SPACE"),
			func(ctx context.Context) error {
				if isEmojiSuggestionEnabled {
					// Select emoji and wait for the candidate window disappear.
					return uiauto.Combine("select emoji suggestion",
						ui.LeftClick(emojiCharFinder),
						ui.WaitUntilGone(emojiCandidateWindowFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), word+" "+emoji),
					)(ctx)
				}
				// Otherwise check emoji suggestion window does not appear in 1s.
				// Sleep is necessary here, otherwise it immediately returns success because of UI reflection delay.
				testing.Sleep(ctx, time.Second)
				return uiauto.Combine("continue to input without emoji suggestion",
					ui.WaitUntilGone(emojiCandidateWindowFinder),
					kb.TypeAction(word),
					util.WaitForFieldTextToBe(tconn, inputField.Finder(), word+" "+word),
				)(ctx)
			},
		)
		return useractions.NewUserAction(
			"validate emoji suggestion",
			action,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: testScenario,
					useractions.AttributeInputField:   string(inputField),
				},
				Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiSuggestion},
			},
		)
	}

	validateLearnMoreUserAction := useractions.NewUserAction(
		"learn more of emoji suggestion",
		uiauto.Combine(`validate "learn more" in emoji suggestion`,
			its.Clear(inputField),
			its.ClickFieldAndWaitForActive(inputField),
			// Use the first data to test "Learn more".
			kb.TypeAction(word),
			kb.AccelAction("SPACE"),
			ui.LeftClick(learnMoreFinder),
			ui.WaitUntilGone(emojiCandidateWindowFinder),
			ossettings.New(tconn).WaitUntilToggleOption(cr, imesettings.EmojiSugestionsOption, true),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: `click "learn more" in emoji suggestion window to launch setting`,
				useractions.AttributeInputField:   string(inputField),
			},
			Tags: []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiSuggestion},
		},
	)

	if err := validateInputUserAction("Emoji suggestion is enabled by default", true).Run(ctx); err != nil {
		s.Fatal("Failed to validate emoji suggestion is enabled by default: ", err)
	}

	if err := imesettings.SetEmojiSuggestions(uc, false).Run(ctx); err != nil {
		s.Fatal("Failed to disable emoji suggestion in OS setting: ", err)
	}

	if err := validateInputUserAction("Emoji suggestion disabled in OS setting", false).Run(ctx); err != nil {
		s.Fatal("Failed to validate input with emoji suggestion disabled: ", err)
	}

	if err := imesettings.SetEmojiSuggestions(uc, true).Run(ctx); err != nil {
		s.Fatal("Failed to enable emoji suggestion in OS setting: ", err)
	}

	if err := validateInputUserAction("Emoji suggestion re-enabled in OS setting", true).Run(ctx); err != nil {
		s.Fatal("Failed to input emoji from suggestion: ", err)
	}

	if err := validateLearnMoreUserAction.Run(ctx); err != nil {
		s.Fatal("Failed to validate learn more of emoji suggestion: ", err)
	}
}
