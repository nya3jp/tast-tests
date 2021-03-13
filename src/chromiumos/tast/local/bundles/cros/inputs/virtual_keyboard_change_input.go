// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
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
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "exp",
			Pre:               pre.VKEnabledTabletExp,
			ExtraSoftwareDeps: []string{"gboard_decoder"},
			ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
		}},
	})
}

func VirtualKeyboardChangeInput(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	const (
		defaultInputMethod       = string(ime.INPUTMETHOD_XKB_US_ENG)
		defaultInputMethodLabel  = "US"
		defaultInputMethodOption = "US keyboard"
		// TODO(crbug/889763) : Update inputMethodOptions once
		// https://crrev.com/c/2519337 is merged.
		updatedInputMethodOption = "English (US)"
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

	inputField := testserver.TextAreaInputField

	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	// Input method changing is done async between front-end ui and background.
	// So nicely to assert both of them to make sure input method changed completely.
	assertInputMethod := func(ctx context.Context, inputMethod, inputMethodLabel string) {
		s.Logf("Wait for current input method label to be %q, %q", inputMethod, inputMethodLabel)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Assert current language using API
			currentInputMethod, err := ime.GetCurrentInputMethod(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get current input method")
			} else if currentInputMethod != ime.IMEPrefix+inputMethod {
				return errors.Errorf("failed to verify current input method. got %q; want %q", currentInputMethod, ime.IMEPrefix+inputMethod)
			}

			imeKeyNode, err := vkb.DescendantNode(ctx, tconn, ui.FindParams{Name: inputMethodLabel})
			if err != nil {
				return errors.Wrapf(err, "failed to wait for language menu label change to %s", inputMethodLabel)
			}
			defer imeKeyNode.Release(ctx)
			return nil
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatal("Failed to assert input method: ", err)
		}
	}

	// Assert default input method.
	assertInputMethod(ctx, defaultInputMethod, defaultInputMethodLabel)

	s.Log("Switch input method with keybaord shortcut Ctrl+Shift+Space")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Ctrl+Shift+Space"); err != nil {
		s.Fatal("Accel(Ctrl+Shift+Space) failed: ", err)
	}

	// Assert new input method after switching with keyboard shortcut.
	assertInputMethod(ctx, inputMethod, InputMethodLabel)

	// Using polling to retry open language menu.
	// Right after changing input method, input view might not respond to js call in a short time.
	// Causing issue "a javascript remote object was not return".
	s.Log("Switch input method on virtual keyboard")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := vkb.TapKey(ctx, tconn, "open keyboard menu"); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to click language menu on vk: ", err)
	}

	languageOptionParams := ui.FindParams{
		Name: defaultInputMethodOption,
	}
	updatedLanguageOptionParams := ui.FindParams{
		Name: updatedInputMethodOption,
	}
	opts := testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}
	if err := ui.StableFindAndClick(ctx, tconn, languageOptionParams, &opts); err != nil {
		if err := ui.StableFindAndClick(ctx, tconn, updatedLanguageOptionParams, &opts); err != nil {
			s.Fatalf("Failed to select language option %s or %s: %v", defaultInputMethodOption, updatedInputMethodOption, err)
		}
	}

	assertInputMethod(ctx, defaultInputMethod, defaultInputMethodLabel)
}
