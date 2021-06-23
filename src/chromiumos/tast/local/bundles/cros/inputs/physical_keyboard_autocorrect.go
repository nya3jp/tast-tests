// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/autocorrect"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAutocorrect,
		Desc:         "Checks that physical keyboard can perform typing with autocorrects",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Pre:          pre.NonVKClamshellReset,
		HardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
		Params: []testing.Param{
			{
				Name: "en_us_1",
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_US_ENG),
					MisspeltWord:  "helol",
					CorrectWord:   "hello",
					UndoMethod:    autocorrect.ViaPopupUsingPK,
				},
			}, {
				Name: "en_us_2",
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_US_ENG),
					MisspeltWord:  "wrold",
					CorrectWord:   "world",
					UndoMethod:    autocorrect.ViaPopupUsingMouse,
				},
			},
			// Test cases for other input methods can be added once the framework
			// supports more than just US-Qwerty layout.
		},
	})
}

func PhysicalKeyboardAutocorrect(ctx context.Context, s *testing.State) {
	testCase := s.Param().(autocorrect.TestCase)
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	imeCode := ime.IMEPrefix + testCase.InputMethodID
	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

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

	setEnabledPKAutocorrectSettings := func(enabled bool) {
		var level = "0"
		if enabled {
			level = "1"
		}
		var settingsAPICall = fmt.Sprintf(
			`chrome.inputMethodPrivate.setSettings(
						 "%s", { "physicalKeyboardAutoCorrectionLevel": %s})`,
			testCase.InputMethodID, level)

		tconn := s.PreValue().(pre.PreData).TestAPIConn
		if err := tconn.Eval(ctx, settingsAPICall, nil); err != nil {
			s.Fatal("Failed to set settings: ", err)
		}
	}

	setEnabledPKAutocorrectSettings(true)
	defer setEnabledPKAutocorrectSettings(false)

	const inputField = testserver.TextAreaInputField
	if err := uiauto.Combine("validate PK autocorrect",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(testCase.MisspeltWord),
		its.WaitForFieldValueToBe(inputField, testCase.MisspeltWord),
		keyboard.TypeAction(" "),
		its.WaitForFieldValueToBe(inputField, testCase.CorrectWord+" "),
	)(ctx); err != nil {
		s.Fatal("Failed to validate PK autocorrect: ", err)
	}

	if err := uiauto.Repeat(len(testCase.CorrectWord)/2+1, keyboard.AccelAction("Left"))(ctx); err != nil {
		s.Fatal("Failed to press Left: ", err)
	}

	ui := uiauto.New(tconn)
	undoWindowFinder := nodewith.ClassName("UndoWindow").Role(role.Window)
	undoButtonFinder := nodewith.Name("Undo").Role(role.Button).Ancestor(undoWindowFinder)

	if err := ui.WaitUntilExists(undoButtonFinder)(ctx); err != nil {
		s.Fatal("Cannot find Undo button: ", err)
	}

	switch testCase.UndoMethod {
	case autocorrect.ViaBackspace:
		// Not applicable for PK. Expect no undo upon Backspace.
		if err := uiauto.Combine("validate PK autocorrect non-undo via Backspace",
			keyboard.AccelAction("Backspace"),
			its.WaitForFieldValueToBe(inputField, testCase.CorrectWord),
		)(ctx); err != nil {
			s.Fatal("Failed to validate PK autocorrect non-undo via Backspace: ", err)
		}
	case autocorrect.ViaPopupUsingPK:
		if err := uiauto.Combine("validate PK autocorrect undo via popup using PK",
			keyboard.AccelAction("Up"),
			keyboard.AccelAction("Enter"),
			its.WaitForFieldValueToBe(inputField, testCase.MisspeltWord+" "),
		)(ctx); err != nil {
			s.Fatal("Failed to validate PK autocorrect undo via popup using PK: ", err)
		}
	case autocorrect.ViaPopupUsingMouse:
		if err := uiauto.Combine("validate PK autocorrect undo",
			ui.LeftClick(undoButtonFinder),
			its.WaitForFieldValueToBe(inputField, testCase.MisspeltWord+" "),
		)(ctx); err != nil {
			s.Fatal("Failed to validate PK autocorrect undo via popup using mouse: ", err)
		}
	}
}
