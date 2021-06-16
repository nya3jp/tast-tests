// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// autocorrectTestCase struct encapsulates parameters for each Autocorrect test.
type autocorrectTestCase struct {
	inputMethodID string
	misspeltWord  string
	correctWord   string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAutocorrect,
		Desc:         "Checks that physical keyboard can perform typing with autocorrects",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "en_us",
				Pre:  pre.NonVKClamshell,
				Val: autocorrectTestCase{
					inputMethodID: string(ime.INPUTMETHOD_XKB_US_ENG),
					misspeltWord:  "helol",
					correctWord:   "hello",
				},
				// Test cases for other input methods can be added once the framework
				// supports more than just US-Qwerty layout.
			},
		},
	})
}

func PhysicalKeyboardAutocorrect(ctx context.Context, s *testing.State) {
	testCase := s.Param().(autocorrectTestCase)
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	imeCode := ime.IMEPrefix + testCase.inputMethodID
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

	var settingsAPICall = fmt.Sprintf(
		`chrome.inputMethodPrivate.setSettings(
			     "%s", { "physicalKeyboardAutoCorrectionLevel": 1})`,
		ime.INPUTMETHOD_XKB_US_ENG)
	if err := tconn.Eval(ctx, settingsAPICall, nil); err != nil {
		s.Fatal("Failed to set settings: ", err)
	}

	const inputField = testserver.TextAreaInputField
	if err := uiauto.Combine("validate PK autocorrect",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(testCase.misspeltWord),
		its.WaitForFieldValueToBe(inputField, testCase.misspeltWord),
		keyboard.TypeAction(" "),
		its.WaitForFieldValueToBe(inputField, testCase.correctWord+" "),
	)(ctx); err != nil {
		s.Fatal("Failed to validate PK autocorrect: ", err)
	}
}
