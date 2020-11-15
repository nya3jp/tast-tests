// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/imesettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodManagement,
		Desc:         "Verifies that user can manage input methods in OS settings",
		Contacts:     []string{"shengjun@chromium.org", "myy@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

func InputMethodManagement(ctx context.Context, s *testing.State) {
	const (
		searchKeyword   = "japanese"                                           // Keyword used to search input method.
		inputMethodName = "Japanese with US keyboard"                          // Input method should be displayed after search.
		inputMethdCode  = ime.IMEPrefix + string(ime.INPUTMETHOD_NACL_MOZC_US) // Input method code of the input method.
	)

	// New language settings UI is behind --enable-features=LanguageSettingsUpdate flag.
	// Will remove this and change it to precondition once it is enabled by default.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=LanguageSettingsUpdate"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to launch OS settings and land at inputs setting page: ", err)
	}

	if err := imesettings.ClickAddInputMethodButton(ctx, tconn); err != nil {
		s.Fatal("Failed to click add input method button: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := imesettings.SearchInputMethod(ctx, tconn, keyboard, searchKeyword, inputMethodName); err != nil {
		s.Fatal("Failed to search input method: ", err)
	}

	if err := imesettings.SelectInputMethod(ctx, tconn, inputMethodName); err != nil {
		s.Fatal("Failed to select input method: ", err)
	}

	if err := imesettings.ClickAddButtonToConfirm(ctx, tconn); err != nil {
		s.Fatal("Failed to click add button to confirm: ", err)
	}

	// Check IME installed through Chrome API.
	if err := ime.WaitForInputMethodInstalled(ctx, tconn, inputMethdCode, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for input method installed: ", err)
	}

	// Remove input method naturally validates that the newly installed input method is shown on UI.
	if err := imesettings.RemoveInputMethod(ctx, tconn, inputMethodName); err != nil {
		s.Fatal("Failed to remove input method from os settings: ", err)
	}

	if err := ime.WaitForInputMethodRemoved(ctx, tconn, inputMethdCode, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for input method removed: ", err)
	}
}
