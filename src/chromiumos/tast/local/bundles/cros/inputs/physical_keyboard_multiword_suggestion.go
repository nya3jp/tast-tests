// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

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
		Func:         PhysicalKeyboardMultiwordSuggestion,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks on device multiword suggestions with physical keyboard typing",
		Contacts:     []string{"curtismcmullan@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools", "group:input-tools-upstream"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.MultiwordEnabledModels...)),
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshellWithMultiwordSuggest,
	})
}

func PhysicalKeyboardMultiwordSuggestion(ctx context.Context, s *testing.State) {
	const (
		inputText    = "ho"
		expectedText = "how are you"
	)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// PK emoji suggestion only works in English(US).
	inputMethod := ime.EnglishUS

	// Activate function checks the current IME. It does nothing if the given input method is already in-use.
	// It is called here just in case IME has been changed in last test.
	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

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
	suggestionWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	ui := uiauto.New(tconn)

	validateAction := uiauto.Combine("accept multiword suggestion",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(inputText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputText),
		ui.WaitUntilExists(suggestionWindowFinder),
		keyboard.AccelAction("Tab"),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
	)

	if err := useractions.NewUserAction(
		"Accept multiword suggestion",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
			},
			Tags: []useractions.ActionTag{useractions.ActionTagMultiwordSuggest},
		},
	).Run(ctx); err != nil {
		s.Fatal("Fail to accept multiword suggestion: ", err)
	}
}
