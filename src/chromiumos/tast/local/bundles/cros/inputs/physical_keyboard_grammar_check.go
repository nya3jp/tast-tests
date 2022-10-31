// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
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
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardGrammarCheck,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks on device grammar check with physical keyboard typing",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:ml_service", "ml_service_ondevice_grammar_check"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		SoftwareDeps: []string{"chrome", "chrome_internal", "ondevice_grammar"},
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.GrammarEnabledModels...)),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				Fixture:           fixture.ClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(pre.GrammarEnabledModels...)),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
		},
	})
}

func PhysicalKeyboardGrammarCheck(ctx context.Context, s *testing.State) {
	const (
		inputText    = "They is student. "
		expectedText = "They are students. "
	)

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	inputField := testserver.TextAreaInputField
	ui := uiauto.New(tconn)
	sentenceTextFinder := nodewith.Name(inputText).Role(role.StaticText)
	grammarWindowFinder := nodewith.ClassName("GrammarSuggestionWindow").Role(role.Window)
	grammarSuggestionButtonFinder := nodewith.Name("are students").Ancestor(grammarWindowFinder).First()

	clickOffsets := [2]int{10, -10}
	i := 0
	validateAction := uiauto.Combine("accept grammar check suggestion",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(inputText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputText),
		// The grammar check can be delayed a few seconds.
		// Retry clicking the sentence area to wait for trigger.
		ui.Retry(5, uiauto.Combine("click sentence to trigger grammar suggestion",
			func(ctx context.Context) error {
				sentenceTextLoc, err := ui.Location(ctx, sentenceTextFinder)
				if err != nil {
					return errors.Wrap(err, "failed to get sentence location")
				}
				// If the cursor is already in the middle of wrong sentence,
				// clicking the same location will not trigger grammar window.
				// Using 2 locations to click alternatively.
				clickLoc := coords.Point{X: sentenceTextLoc.CenterX() + clickOffsets[i%2], Y: sentenceTextLoc.CenterY()}
				i++
				return ui.MouseClickAtLocation(0, clickLoc)(ctx)
			},
			ui.WithTimeout(3*time.Second).WaitUntilExists(grammarWindowFinder),
		)),
		ui.LeftClick(grammarSuggestionButtonFinder),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
	)

	if err := uiauto.UserAction(
		"Accept grammar check suggestion",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
				useractions.AttributeFeature:    useractions.FeatureGrammarCheck,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to accept grammar check suggestion: ", err)
	}
}
