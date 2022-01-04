// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"math/rand"
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
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream", "informational"},
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
	sentenseTextFinder := nodewith.Name(inputText).Role(role.StaticText)
	grammarWindowFinder := nodewith.ClassName("GrammarSuggestionWindow").Role(role.Window)
	grammarSuggestionButtonFinder := nodewith.ClassName("SuggestionView").Role(role.Button).Ancestor(grammarWindowFinder)

	validateAction := uiauto.Combine("accept grammar check suggestion",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(inputText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputText),
		// The grammar check can be delayed a few seconds.
		// Retry clicking the sentense area to wait for trigger.
		ui.Retry(5, uiauto.Combine("click sentense to trigger grammar suggestion",
			func(ctx context.Context) error {
				sentenseTextLoc, err := ui.Location(ctx, sentenseTextFinder)
				if err != nil {
					return errors.Wrap(err, "failed to get sentense location")
				}
				// If the cursor is already in the middle of wrong sentense,
				// clicking the same location will not trigger grammar window.
				// Using random location to click around.
				sentenseCenterPoint := sentenseTextLoc.CenterPoint()
				rand.Seed(time.Now().UnixNano())
				clickLoc := coords.Point{X: sentenseCenterPoint.X - 20 + rand.Intn(40), Y: sentenseCenterPoint.Y}
				return ui.MouseClickAtLocation(0, clickLoc)(ctx)
			},
			ui.WithTimeout(3*time.Second).WaitUntilExists(grammarWindowFinder),
		)),
		ui.LeftClick(grammarSuggestionButtonFinder),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
	)

	if err := useractions.NewUserAction(
		"Accept grammar check suggestion",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
			},
			Tags: []useractions.ActionTag{useractions.ActionTagGrammarCheck},
		},
	).Run(ctx); err != nil {
		s.Fatal("Fail to accept grammar check suggestion: ", err)
	}
}
