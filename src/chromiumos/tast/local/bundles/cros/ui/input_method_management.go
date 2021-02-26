// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
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
		inputMethodCode = ime.IMEPrefix + string(ime.INPUTMETHOD_NACL_MOZC_US) // Input method code of the input method.
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
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to launch OS settings and land at inputs setting page: ", err)
	}
	if err := uiauto.Combine("test input method management",
		settings.ClickAddInputMethodButton(),
		settings.SearchInputMethod(keyboard, searchKeyword, inputMethodName),
		settings.SelectInputMethod(inputMethodName),
		settings.ClickAddButtonToConfirm(),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodInstalled(ctx, tconn, inputMethodCode, 10*time.Second)
		},
		settings.RemoveInputMethod(inputMethodName),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodRemoved(ctx, tconn, inputMethodCode, 10*time.Second)
		},
	)(ctx); err != nil {
		s.Fatal("Failed to test input method management: ", err)
	}
}
