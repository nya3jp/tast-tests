// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardJapaneseInputs,
		Desc:         "Checks switching between Romaji and Kana mode for Japanese inputs",
		Contacts:     []string{"myy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
	})
}

func VirtualKeyboardJapaneseInputs(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	const (
		defaultInputMethod       = "xkb:us::eng"
		defaultInputMethodLabel  = "US"
		defaultInputMethodOption = "US keyboard"
		language                 = "ja"
		inputMethod              = "nacl_mozc_jp"
		inputMethodLabel         = "あ"
	)

	// Adds Japanese language and input method.
	if err := ime.EnableLanguage(ctx, tconn, language); err != nil {
		s.Fatalf("Failed to enable the language %q: %v", language, err)
	}

	if err := ime.AddInputMethod(ctx, tconn, vkb.ImePrefix+inputMethod); err != nil {
		s.Fatalf("Failed to enable the IME %q: %v", inputMethod, err)
	}

	// Opens Japanese input methods options page
	optionPage, err := cr.NewConn(ctx, "chrome-extension://jkghodnilhceideoidjikpgommlajknk/mozc_option.html")
	if err != nil {
		s.Error("Failed to open page: ", err)
	}
	defer optionPage.Close()

	// Waits for page to load.
	testing.Sleep(ctx, 3*time.Second)

	// Gets omnibox
	params := ui.FindParams{
		Role:       ui.RoleTypeTextField,
		Attributes: map[string]interface{}{"inputType": "url"},
	}
	omnibox, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for the omnibox: ", err)
	}
	defer omnibox.Release(ctx)

	// Gets header
	header, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeHeader}, 3*time.Second)
	if err != nil {
		s.Fatal("Failed to find header: ", err)
	}

	focusOnOptionPage := func(ctx context.Context) {
		s.Log("Focusing on Option Page")
		if err := header.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click the header: ", err)
		}
	}

	clickOmnibox := func(ctx context.Context) {
		s.Log("Clicking omnibox")
		if err := omnibox.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click the omnibox: ", err)
		}
	}

	// Input method changing is done async between front-end ui and background.
	// So nicely to assert both of them to make sure input method changed completely.
	assertInputMethod := func(ctx context.Context, inputMethod, inputMethodLabel string) {
		s.Logf("Waiting for %v virtual keyboard to show", inputMethod)
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
		}

		s.Logf("Waiting for current input method label to be %q, %q", inputMethod, inputMethodLabel)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Assert current language using API
			currentInputMethod, err := vkb.GetCurrentInputMethod(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get current input method")
			} else if currentInputMethod != vkb.ImePrefix+inputMethod {
				return errors.Errorf("failed to verify current input method. got %q; want %q", currentInputMethod, vkb.ImePrefix+inputMethod)
			}
			keyboard, err := vkb.VirtualKeyboard(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "virtual keyboard does not show")
			}
			defer keyboard.Release(ctx)

			if _, err := keyboard.Descendants(ctx, ui.FindParams{Name: inputMethodLabel}); err != nil {
				return errors.Wrapf(err, "failed to wait for language menu label change to %s", inputMethodLabel)
			}
			return nil
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatal("Failed to assert input method: ", err)
		}
	}

	waitForVirtualKeyboard := func(ctx context.Context) {
		s.Log("Waiting for the virtual keyboard to show")
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
		}

		s.Log("Waiting for the virtual keyboard to render buttons")
		if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
		}
	}

	hideVirtualKeyboard := func(ctx context.Context) {
		s.Log("Hiding virtual keyboard")
		if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
			s.Error("Failed to hide the virtual keyboard: ", err)
		}

		if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
			s.Error("Failed to wait for the virtual keyboard to hide: ", err)
		}
	}

	clearOmnibox := func(ctx context.Context) {
		s.Log("Clearing omnibox content")
		if err := vkb.TapKeys(ctx, tconn, []string{"backspace"}); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}
	}

	switchInputMode := func(inputMode string) {
		s.Log("Switching input mode to ", inputMode)
		if err := optionPage.Eval(ctx,
			fmt.Sprintf(`document.getElementById('preedit_method').value = '%s';
			document.getElementById('preedit_method').dispatchEvent(new Event('change'));`, inputMode), nil); err != nil {
			s.Errorf("Failed to update input mode to %s: %v", inputMode, err)
		}
	}

	assertInputMode := func(ctx context.Context, expectedInputMode string, typingKeys []string, expectedOutput string) {
		var inputMode string
		if err := optionPage.Eval(ctx, ` document.getElementById('preedit_method').value`, &inputMode); err != nil {
			s.Error(inputMode)
		}

		if strings.Contains(inputMode, expectedInputMode) {
			clickOmnibox(ctx)

			waitForVirtualKeyboard(ctx)

			if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
				s.Fatal("Failed to input with virtual keyboard: ", err)
			}

			// Value change can be a bit delayed after input.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				omniboxValue, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "OmniboxResultView"}, 2*time.Second)
				if err != nil {
					return err
				}
				if omniboxValue.Name != expectedOutput {
					return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", omniboxValue.Name, expectedOutput)
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				s.Error("Failed to input with virtual keyboard: ", err)
			}

			clearOmnibox(ctx)
			hideVirtualKeyboard(ctx)
			// Need to focus on option page and hide keyboard again for the keyboard to reload properly.
			focusOnOptionPage(ctx)
			hideVirtualKeyboard(ctx)
		}
	}

	// Brings up virtual keyboard
	clickOmnibox(ctx)
	waitForVirtualKeyboard(ctx)
	clearOmnibox(ctx)

	// Asserts default input method.
	assertInputMethod(ctx, defaultInputMethod, defaultInputMethodLabel)

	s.Log("Switch input method with keyboard shortcut Ctrl+Shift+Space")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Ctrl+Shift+Space"); err != nil {
		s.Fatal("Accel(Ctrl+Shift+Space) failed: ", err)
	}

	// Asserts japanese input method after switching with keyboard shortcut.
	assertInputMethod(ctx, inputMethod, inputMethodLabel)

	const romajiInputMode = "ROMAN"
	var romajiTypingKeys = []string{"a"}
	const romajiExpectedOmniboxOutput = "あ search"
	// Initial input mode is Romaji
	assertInputMode(ctx, romajiInputMode, romajiTypingKeys, romajiExpectedOmniboxOutput)

	// Switches to Kana
	const kanaInputMode = "KANA"
	var kanaTypingKeys = []string{"ち"}
	const kanaExpectedOmniboxOutput = "ち search"
	switchInputMode(kanaInputMode)
	assertInputMode(ctx, kanaInputMode, kanaTypingKeys, kanaExpectedOmniboxOutput)

	// Switches back to Romaji
	switchInputMode(romajiInputMode)
	assertInputMode(ctx, romajiInputMode, romajiTypingKeys, romajiExpectedOmniboxOutput)
}
