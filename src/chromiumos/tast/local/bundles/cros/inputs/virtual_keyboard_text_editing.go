// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTextEditing,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the virtual keyboard can insert and delete text after clicking between different text fields",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func VirtualKeyboardTextEditing(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	ui := uiauto.New(tconn)
	vkbCtx := vkb.NewContext(cr, tconn)

	inputMethod := ime.EnglishUS
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	inputField := testserver.TextAreaInputField
	noCorrectionInputField := testserver.TextAreaNoCorrectionInputField
	inputTextFinder := nodewith.Role(role.InlineTextBox).Ancestor(inputField.Finder())
	noCorrectionInputFieldFinder := nodewith.Role(role.InlineTextBox).Ancestor(noCorrectionInputField.Finder())

	clickTextRightBound := func(textFinder *nodewith.Finder, endIndex int) uiauto.Action {
		return func(ctx context.Context) error {
			textBounds, err := ui.BoundsForRange(ctx, textFinder, 0, endIndex)
			if err != nil {
				return errors.Wrap(err, "failed to get text location")
			}
			return ui.MouseClickAtLocation(0, textBounds.RightCenter())(ctx)
		}
	}

	validateAction := uiauto.Combine("edit text using virtual keyboard",
		// Edit text after swapping focus between text fields.
		its.ClickFieldUntilVKShown(inputField),
		vkbCtx.TapKeys(strings.Split("Abcdfg", "")),
		its.ClickFieldUntilVKShown(noCorrectionInputField),
		vkbCtx.TapKeys(strings.Split("abd", "")),
		clickTextRightBound(inputTextFinder, 4),
		vkbCtx.TapKeys(strings.Split("de", "")),
		clickTextRightBound(noCorrectionInputFieldFinder, 2),
		vkbCtx.TapKeys(strings.Split("c", "")),
		clickTextRightBound(inputTextFinder, 5),
		vkbCtx.TapKey("backspace"),
		clickTextRightBound(noCorrectionInputFieldFinder, 2),
		vkbCtx.TapKey("space"),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Abcdefg"),
		util.WaitForFieldTextToBe(tconn, noCorrectionInputField.Finder(), "ab cd"),

		// Edit text while focused in text field.
		clickTextRightBound(inputTextFinder, 7),
		vkbCtx.TapKeys(strings.Split("hjij", "")),
		clickTextRightBound(inputTextFinder, 9),
		vkbCtx.TapKey("backspace"),
		clickTextRightBound(inputTextFinder, 5),
		vkbCtx.TapKey("space"),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Abcde fghij"),
	)

	if err := uiauto.UserAction("Edit text using virtual keyboard",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:    useractions.FeatureVKTyping,
				useractions.AttributeInputField: string(inputField),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to edit text using virtual keyboard: ", err)
	}
}
