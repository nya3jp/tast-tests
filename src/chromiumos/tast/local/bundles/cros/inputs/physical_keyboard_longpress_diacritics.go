// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardLongpressDiacritics,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks diacritics on long-press with physical keyboard typing",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      2 * time.Minute,
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVKWithDiacriticsOnPKLongpress,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVKWithDiacriticsOnPKLongpress,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
		},
	})
}

func PhysicalKeyboardLongpressDiacritics(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// PK longpress diacritics only works in English(US).
	inputMethod := ime.EnglishUS

	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	const inputField = testserver.TextInputField

	const (
		longpressKeyChar = "a"
		diacritic        = "à"
	)

	candidateWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	suggestionCharFinder := nodewith.Name(diacritic).Ancestor(candidateWindowFinder).First()
	ui := uiauto.New(tconn)
	actionName := "PK longpress to insert diacritics"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			its.ClickFieldAndWaitForActive(inputField),
			// Simulate a held down key press.
			kb.AccelPressAction(longpressKeyChar),
			ui.WaitUntilExists(candidateWindowFinder),
			kb.AccelReleaseAction(longpressKeyChar),
			ui.LeftClick(suggestionCharFinder),
			ui.WaitUntilGone(candidateWindowFinder),
			its.ValidateResult(inputField, diacritic),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureLongpressDiacritics,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate diacritics on PK longpress: ", err)
	}

}
