// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks on device grammar check with physical keyboard typing",
		Contacts:     []string{"jiwan@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.GrammarEnabledModels...)),
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshellWithGrammarCheck,
	})
}

func PhysicalKeyboardGrammarCheck(ctx context.Context, s *testing.State) {
	const (
		inputText    = "They is student."
		expectedText = "They are students."
	)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	inputField := testserver.TextAreaInputField
	ui := uiauto.New(tconn)
	sentenceTextFinder := nodewith.Name(inputText).Role(role.StaticText)
	grammarWindowFinder := nodewith.ClassName("GrammarSuggestionWindow").Role(role.Window)
	grammarSuggestionButtonFinder := nodewith.ClassName("SuggestionView").Role(role.Button).Ancestor(grammarWindowFinder)

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
