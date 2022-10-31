// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardChangeInput,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that changing input method in different ways",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.ChinesePinyin}),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "tablet",
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "a11y",
				Fixture:           fixture.ClamshellVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "tablet_lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
			{
				Name:              "a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func VirtualKeyboardChangeInput(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := ime.ChinesePinyin
	typingTestData, ok := data.TypingMessageHello.GetInputData(inputMethod)
	if !ok {
		s.Fatalf("Test Data for input method %v does not exist", inputMethod)
	}

	if err := inputMethod.Install(tconn)(ctx); err != nil {
		s.Fatalf("Failed to install input method %q: %v", inputMethod, err)
	}

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	ui := uiauto.New(tconn)
	vkbctx := vkb.NewContext(cr, tconn)

	inputField := testserver.TextAreaInputField
	inputMethodOption := vkb.NodeFinder.Name(inputMethod.Name).Role(role.StaticText)
	// String is changing to uppercase, so allow either o.
	vkLanguageMenuFinder := vkb.KeyFinder.NameRegex(regexp.MustCompile("[oO]pen keyboard menu"))

	validateAction := uiauto.Combine("verify changing input method on virtual keyboard",
		// Switch IME using virtual keyboard language menu.
		its.ClickFieldUntilVKShown(inputField),
		ui.LeftClick(vkLanguageMenuFinder),
		ui.LeftClick(inputMethodOption),
		ui.WaitUntilExists(vkb.NodeFinder.Name(inputMethod.ShortLabel).Role(role.StaticText)),

		// Validate current input method change.
		inputMethod.WaitUntilActivated(tconn),

		// Validate typing test.
		vkbctx.TapKeysIgnoringCase(typingTestData.CharacterKeySeq),
		vkbctx.SelectFromSuggestion(typingTestData.ExpectedText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), typingTestData.ExpectedText),
	)

	if err := uiauto.UserAction("Switch input method on VK",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf("Change input method to %q", inputMethod.Name),
				useractions.AttributeFeature:      useractions.FeatureIMEManagement,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify changing input method: ", err)
	}
}
