// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardChangeInput,
		Desc:         "Checks that changing input method in different ways",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream", "informational"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func VirtualKeyboardChangeInput(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardChangeInput.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	const (
		defaultInputMethod       = string(ime.INPUTMETHOD_XKB_US_ENG)
		defaultInputMethodLabel  = "US"
		defaultInputMethodOption = "English (US)"
		language                 = "fr-FR"
		inputMethod              = string(ime.INPUTMETHOD_XKB_FR_FRA)
		InputMethodLabel         = "FR"
	)

	if err := ime.AddInputMethod(ctx, tconn, ime.IMEPrefix+inputMethod); err != nil {
		s.Fatal("Failed to add input method: ", err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	s.Log("Switch input method with keybaord shortcut Ctrl+Shift+Space")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	inputField := testserver.TextAreaInputField
	defaultInputMethodOptionFinder := vkb.NodeFinder.Name(defaultInputMethodOption).Role(role.StaticText)
	vkLanguageMenuFinder := vkb.KeyFinder.Name("open keyboard menu")
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("verify changing input method with shortcut and virtual keyboard UI",
		// Trigger VK and assert default IME.
		its.ClickFieldUntilVKShown(inputField),
		assertInputMethod(tconn, defaultInputMethod, defaultInputMethodLabel),
		// Switch IME with shortcut and assert IME changed successfully.
		keyboard.AccelAction("Ctrl+Shift+Space"),
		assertInputMethod(tconn, inputMethod, InputMethodLabel),
		// Switch back to default IME on virtual keyboard UI.
		ui.LeftClick(vkLanguageMenuFinder),
		ui.LeftClick(defaultInputMethodOptionFinder),
		assertInputMethod(tconn, defaultInputMethod, defaultInputMethodLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to verify changing input method: ", err)
	}
}

// assertInputMethod asserts current input method.
// Input method changing is done async between front-end ui and background.
// So nicely to assert both of them to make sure input method changed completely.
func assertInputMethod(tconn *chrome.TestConn, inputMethod, inputMethodLabel string) uiauto.Action {
	ui := uiauto.New(tconn)
	assertAction := func(ctx context.Context) error {
		currentInputMethod, err := ime.GetCurrentInputMethod(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get current input method")
		} else if currentInputMethod != ime.IMEPrefix+inputMethod {
			return errors.Errorf("failed to verify current input method. got %q; want %q", currentInputMethod, ime.IMEPrefix+inputMethod)
		}

		if err := ui.WaitUntilExists(vkb.NodeFinder.Name(inputMethodLabel).Role(role.StaticText))(ctx); err != nil {
			return errors.Wrapf(err, "failed to wait for language menu label change to %s", inputMethodLabel)
		}
		return nil
	}
	return ui.WithInterval(1*time.Second).Retry(10, assertAction)
}
