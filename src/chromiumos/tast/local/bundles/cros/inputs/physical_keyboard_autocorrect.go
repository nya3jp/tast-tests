// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/autocorrect"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingPK,
				},
			}, {
				Name: "en_us_2",
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
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := testCase.InputMethod
	s.Logf("Set current input method to: %q", inputMethod)

	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %q: %v: ", inputMethod, err)
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

	defer func() {
		if err := inputMethod.ResetSettings(tconn)(cleanupCtx); err != nil {
			// Only log errors in cleanup.
			s.Log("Failed to reset IME settings: ", err)
		}
	}()

	const inputField = testserver.TextAreaInputField
	if err := uiauto.Combine("validate PK autocorrect",
		inputMethod.SetPKAutoCorrection(tconn, ime.AutoCorrectionModest),
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(testCase.MisspeltWord),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord),
		keyboard.TypeAction(" "),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.CorrectWord+" "),
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
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.CorrectWord),
		)(ctx); err != nil {
			s.Fatal("Failed to validate PK autocorrect non-undo via Backspace: ", err)
		}
	case autocorrect.ViaPopupUsingPK:
		if err := uiauto.Combine("validate PK autocorrect undo via popup using PK",
			keyboard.AccelAction("Up"),
			keyboard.AccelAction("Enter"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord+" "),
		)(ctx); err != nil {
		}
	case autocorrect.ViaPopupUsingMouse:
		if err := uiauto.Combine("validate PK autocorrect undo",
			ui.LeftClick(undoButtonFinder),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord+" "),
		)(ctx); err != nil {
			s.Fatal("Failed to validate PK autocorrect undo via popup using mouse: ", err)
		}
	}
}
