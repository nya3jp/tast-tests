// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/autocorrect"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
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
		Func:         PhysicalKeyboardAutocorrect,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that physical keyboard can perform typing with autocorrects",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
		Params: []testing.Param{
			{
				Name:      "en_us_1",
				Pre:       pre.NonVKClamshellReset,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingPK,
				},
			},
			{
				Name:      "en_us_2",
				Pre:       pre.NonVKClamshellReset,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "wrold",
					CorrectWord:  "world",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
			},
			{
				Name:      "en_us_1_fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingPK,
				},
			},
			{
				Name:      "en_us_2_fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "wrold",
					CorrectWord:  "world",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
			},
			// Test cases for other input methods can be added once the framework
			// supports more than just US-Qwerty layout.
		},
	})
}

func PhysicalKeyboardAutocorrect(ctx context.Context, s *testing.State) {
	testCase := s.Param().(autocorrect.TestCase)
	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := testCase.InputMethod
	s.Logf("Set current input method to: %q", inputMethod)

	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %q: %v: ", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	defer func(ctx context.Context) {
		if err := inputMethod.ResetSettings(tconn)(ctx); err != nil {
			// Only log errors in cleanup.
			s.Log("Failed to reset IME settings: ", err)
		}
	}(cleanupCtx)

	const inputField = testserver.TextAreaInputField

	validatePKAutocorrectAction := uiauto.Combine("validate PK autocorrect",
		inputMethod.SetPKAutoCorrection(tconn, ime.AutoCorrectionModest),
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(testCase.MisspeltWord),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord),
		keyboard.TypeAction(" "),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.CorrectWord+" "),
	)

	if err := uiauto.UserAction("PK autocorrect",
		validatePKAutocorrectAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: fmt.Sprintf(`correct %q to %q`, testCase.MisspeltWord, testCase.CorrectWord),
				useractions.AttributeFeature:      useractions.FeatureAutoCorrection,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate PK autocorrect: ", err)
	}

	ui := uiauto.New(tconn)
	undoWindowFinder := nodewith.ClassName("UndoWindow").Role(role.Window)
	undoButtonFinder := nodewith.Name("Undo").Role(role.Button).Ancestor(undoWindowFinder)

	triggerUndoAction := uiauto.Combine("press left button to trigger AC undo",
		uiauto.Repeat(len(testCase.CorrectWord)/2+1, keyboard.AccelAction("Left")),
		ui.WaitUntilExists(undoButtonFinder),
	)

	if err := uiauto.UserAction("press LEFT key to trigger AC undo",
		triggerUndoAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
				useractions.AttributeFeature:    useractions.FeatureAutoCorrection,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to trigger AC undo: ", err)
	}

	var testScenario string
	var undoAction uiauto.Action
	switch testCase.UndoMethod {
	case autocorrect.ViaBackspace:
		// Not applicable for PK. Expect no undo upon Backspace.
		testScenario = "PK autocorrect non-undo via Backspace"
		undoAction = uiauto.Combine(testScenario,
			keyboard.AccelAction("Backspace"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.CorrectWord),
		)
	case autocorrect.ViaPopupUsingPK:
		testScenario = "PK autocorrect undo via popup using PK"
		undoAction = uiauto.Combine(testScenario,
			keyboard.AccelAction("Up"),
			keyboard.AccelAction("Enter"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord+" "),
		)
	case autocorrect.ViaPopupUsingMouse:
		testScenario = "PK autocorrect undo via popup using mouse"
		undoAction = uiauto.Combine(testScenario,
			ui.LeftClick(undoButtonFinder),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord+" "),
		)
	}

	if err := uiauto.UserAction("PK autocorrect undo",
		undoAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: testScenario,
				useractions.AttributeFeature:      useractions.FeatureAutoCorrection,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate autocorrect undo: ", err)
	}
}
