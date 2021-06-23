// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Fixture:      "chromeLoggedInForInputs",
		Timeout:      5 * time.Minute,
	})
}

func PhysicalKeyboardEnglishTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "PhysicalKeyboardEnglishTyping.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

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

	var subtests = []util.InputEval{
		{
			TestName:     "Mixed (alphanumeric, symbols, enter) typing",
			InputFunc:    keyboard.TypeAction("Hello!\nTesting 123."),
			ExpectedText: "Hello!\nTesting 123.",
		}, {
			TestName: "Backspace",
			InputFunc: uiauto.Combine("type a string and Backspace",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Backspace"),
			),
			ExpectedText: "ab",
		}, {
			TestName: "Ctrl+Backspace",
			InputFunc: uiauto.Combine("type a string and Ctrl+Backspace",
				keyboard.TypeAction("hello world"),
				keyboard.AccelAction("Ctrl+Backspace"),
			),
			ExpectedText: "hello ",
		}, {
			TestName: "Editing middle of text",
			InputFunc: uiauto.Combine("type strings and edit in the middle of text",
				keyboard.TypeAction("abc"),
				keyboard.AccelAction("Left"),
				keyboard.AccelAction("Backspace"),
				keyboard.TypeAction("bc ab"),
			),
			ExpectedText: "abc abc",
		},
	}

	var inputField = testserver.TextAreaInputField

	for _, subtest := range subtests {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtest.TestName)

			if err := its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate %s: %v", subtest.TestName, err)
			}
		})
	}
}
