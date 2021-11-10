// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardIMECUJ,
		Desc:         "Physical keyboard CUJs with different input method",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Pre:          pre.NonVKClamshell,
		Params: []testing.Param{
			{
				Name: "english_uk",
				Val:  ime.EnglishUK,
			},
		},
	})
}

func PhysicalKeyboardIMECUJ(ctx context.Context, s *testing.State) {
	im := s.Param().(ime.InputMethod)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard device: ", err)
	}
	defer kb.Close()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	uc, err := inputactions.NewInputsUserContext(ctx, s, cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create user context: ", err)
	}

	// CUJ: Add input method in OS settings.
	if err := inputactions.AddInputMethodInOSSettings(uc, kb, im).Run(ctx); err != nil {
		s.Fatalf("Failed to add input method %q in OS settings: %v", im, err)
	}

	// CUJ: Switch input method with PK shortcut.
	if err := inputactions.SwitchToInputMethodWithShortcut(uc, kb, im).Run(ctx); err != nil {
		s.Fatalf("Failed to add input method %q in OS settings: %v", im, err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// CUJ subtest: Input Emoji with Emoji Picker in textarea.
	its.InputEmojiWithEmojiPicker(uc, kb, testserver.TextAreaInputField).RunAsSubTest(ctx, s, false)

	// emojiSuggestionSupportedIMEs := []ime.InputMethod{ime.EnglishUK, ime.EnglishUS}
	// CUJ subtest: Input Emoji from suggestion in text field.
	its.InputEmojiFromSuggestion(uc, kb, testserver.TextInputField).RunAsSubTest(ctx, s, false)

	// CUJ: remove input method from OS settings.
	if err := inputactions.RemoveInputMethodInOSSettings(uc, im).Run(ctx); err != nil {
		s.Fatalf("Failed to remove input method %q in OS settings: %v", im, err)
	}
}
