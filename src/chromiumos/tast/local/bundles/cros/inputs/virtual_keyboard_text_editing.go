// Copyright 2022 The ChromiumOS Authors.
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
		Desc:         "Checks text editing using virtual keyboard",
		Contacts:     []string{"michellegc@google.com", "essential-inputs-team@google.com"},
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
				ExtraSoftwareDeps: []string{"lacros"},
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

	inputField := testserver.TextAreaNoCorrectionInputField
	textFinder := nodewith.Role(role.InlineTextBox).Ancestor(inputField.Finder())
	TextFieldHeaderFinder := nodewith.Name("textAreaNoCorrectionInputFieldHeader")

	clickRightBound := func(rightIndex int) uiauto.Action {
		return func(ctx context.Context) error {
			textBounds, err := ui.BoundsForRange(ctx, textFinder, 0, rightIndex)
			if err != nil {
				return errors.Wrap(err, "failed to get text location")
			}
			return ui.MouseClickAtLocation(0, textBounds.RightCenter())(ctx)
		}
	}

	validateAction := uiauto.Combine("edit text using virtual keyboard",
		its.ClickFieldUntilVKShown(inputField),
		vkbCtx.TapKeys(strings.Split("abcdfg", "")),

		// Edit text after unfocusing then refocusing into text field.
		ui.LeftClick(TextFieldHeaderFinder),
		clickRightBound(4),
		vkbCtx.TapKeys([]string{"d", "e"}),
		ui.LeftClick(TextFieldHeaderFinder),
		clickRightBound(5),
		vkbCtx.TapKey("backspace"),
		ui.LeftClick(TextFieldHeaderFinder),
		clickRightBound(2),
		vkbCtx.TapKey("space"),

		// Edit text while focused in text field.
		clickRightBound(8),
		vkbCtx.TapKeys([]string{"i", "h"}),
		clickRightBound(9),
		vkbCtx.TapKey("backspace"),
		clickRightBound(5),
		vkbCtx.TapKey("space"),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "ab cd efgh"),
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
