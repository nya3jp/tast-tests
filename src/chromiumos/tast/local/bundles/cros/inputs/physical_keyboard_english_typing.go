// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEnglishTyping,
		Desc:         "Checks that physical keyboard can perform basic typing",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
	})
}

func PhysicalKeyboardEnglishTyping(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	imeCode := ime.IMEPrefix + string(ime.INPUTMETHOD_XKB_US_ENG)

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

	type testData struct {
		testName     string
		inputFunc    uiauto.Action
		expectedText string
	}

	var subtests = []testData{
		{
			testName:     "Mixed (alphanumeric, symbols, enter) typing",
			inputFunc:    keyboard.TypeAction("Hello!\nTesting 123."),
			expectedText: "Hello!\nTesting 123.",
		}, {
			testName: "Backspace",
			inputFunc: uiauto.Combine("type a string and Backspace",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Backspace"),
			),
			expectedText: "ab",
		}, {
			testName: "Ctrl+Backspace",
			inputFunc: uiauto.Combine("type a string and Ctrl+Backspace",
				keyboard.TypeAction("hello world"),
				keyboard.AccelAction("Ctrl+Backspace"),
			),
			expectedText: "hello ",
		}, {
			testName: "Editing middle of text",
			inputFunc: uiauto.Combine("type strings and edit in the middle of text",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Left"),
				keyboard.AccelAction("Backspace"),
				keyboard.TypeAction("bc ab"),
			),
			expectedText: "abc abc",
		},
	}

	var inputField = testserver.TextAreaInputField

	for _, subtest := range subtests {
		s.Run(ctx, subtest.testName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			if err := its.ValidateInputOnField(inputField, subtest.inputFunc, subtest.expectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate %s: %v", subtest.testName, err)
			}
		})
	}
}
