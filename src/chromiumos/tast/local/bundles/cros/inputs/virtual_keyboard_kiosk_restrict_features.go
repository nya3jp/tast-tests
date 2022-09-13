// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardKioskRestrictFeatures,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that restrict features functionality of extension API works in kiosk mode",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.KioskVK,
			},
			{
				Name:    "lacros",
				Fixture: fixture.LacrosKioskVK,
			},
		},
	})
}

func VirtualKeyboardKioskRestrictFeatures(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(fixture.InputsKioskFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.InputsKioskFixtData).UserContext

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField

	if err := tconn.Eval(ctx, `chrome.virtualKeyboard.restrictFeatures({
			             autoCompleteEnabled: false,
			             autoCorrectEnabled: false,
			             spellCheckEnabled: false,
			             voiceInputEnabled: false,
			             handwritingEnabled: false})`, nil); err != nil {
		s.Fatal("Failed to set virtual keyboard features disabled (via chrome.virtualKeyboard.restrictFeatures): ", err)
	}

	defer func() {
		// Unset the restrictions after test.
		if err := tconn.Eval(ctx, `chrome.virtualKeyboard.restrictFeatures({
			             autoCompleteEnabled: true,
			             autoCorrectEnabled: true,
			             spellCheckEnabled: true,
			             voiceInputEnabled: true,
			             handwritingEnabled: true})`, nil); err != nil {
			s.Fatal("Failed to set virtual keyboard features enabled (via chrome.virtualKeyboard.restrictFeatures): ", err)
		}
	}()

	actionName := "Testing different disabled features in Kiosk mode"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			ui.WaitUntilExists(inputField.Finder()),
			ui.MakeVisible(inputField.Finder()),
			vkbCtx.ClickUntilVKShown(inputField.Finder()),
			vkbCtx.TapKeysIgnoringCase(strings.Split("studnets", "")),
			// Check if there are no suggestions.
			ui.EnsureGoneFor(vkb.KeyFinder.Name("Students").HasClass("sk"), 2*time.Second),
			// Check that it is not automatically corrected to "The" after "space".
			vkbCtx.TapKey("space"),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "studnets "),
			// Check if clicking on handwriting button doesn't open handwriting canvas.
			ui.LeftClick(vkb.KeyFinder.NameStartingWith("switch to handwriting")),
			ui.EnsureGoneFor(vkb.NodeFinder.Role(role.Canvas), 2*time.Second),
			// Check that the voice typing button doesn't display voice input screen.
			ui.LeftClick(vkb.KeyFinder.NameStartingWith("Voice")),
			ui.EnsureGoneFor(vkb.NodeFinder.HasClass("voice-view"), 2*time.Second),
			// Check that you can still type after those two buttons are pressed
			// (confirming no handwriting/voice got activated).
			vkbCtx.TapKeysIgnoringCase(strings.Split("studying", "")),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "studnets studying"),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:      useractions.FeatureVKTyping,
				useractions.AttributeTestScenario: "Testing different disabled features (autocorrect, suggestions, handwriting, voice) in Kiosk mode",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate that relevant features are restricted: ", err)
	}
}
