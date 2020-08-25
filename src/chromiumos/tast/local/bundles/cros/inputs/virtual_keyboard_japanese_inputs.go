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
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
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
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
		}},
	})
}

func VirtualKeyboardJapaneseInputs(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.Region("jp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := vkb.SetCurrentInputMethod(ctx, tconn, "nacl_mozc_jp"); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}

	s.Log("Opening Japanese IME options page")
	optionPage, err := cr.NewConn(ctx, "chrome-extension://jkghodnilhceideoidjikpgommlajknk/mozc_option.html")
	if err != nil {
		s.Error("Failed to open Japanese IME options page: ", err)
	}
	defer optionPage.Close()

	type inputMode struct {
		name    string
		typeKey string
		output  string
	}

	romajiInput := inputMode{
		name:    "ROMAN",
		typeKey: "a",
		output:  "あ",
	}

	kanaInput := inputMode{
		name:    "KANA",
		typeKey: "ち",
		output:  "ち",
	}

	assertInputMode := func(mode inputMode) {
		s.Log("Asserting input mode is ", mode.name)
		// Using omnibox to verify input mode.
		params := ui.FindParams{
			Role:       ui.RoleTypeTextField,
			Attributes: map[string]interface{}{"inputType": "url"},
		}
		omnibox, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to wait for the omnibox: ", err)
		}
		defer omnibox.Release(ctx)

		if err := omnibox.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click the omnibox: ", err)
		}

		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
		}

		if err := vkb.TapKey(ctx, tconn, mode.typeKey); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}

		// Value change can be a bit delayed after input.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if omniboxValue, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "OmniboxResultView"}, 2*time.Second); err != nil {
				return err
			} else if !strings.Contains(omniboxValue.Name, mode.output) {
				return errors.Errorf("unexpected output found: got %s; want %s", omniboxValue.Name, mode.output)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}

		if err := vkb.TapKey(ctx, tconn, "Backspace"); err != nil {
			s.Fatal("Failed to delete with virtual keyboard: ", err)
		}
	}

	switchInputMode := func(mode inputMode) {
		s.Log("Switching input mode to ", mode.name)

		// Click page header to deactive virtualkeyboard.
		// Note: vkb.HideVirtualKeyboard() will not trigger reloading of setting changes.
		header, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeInlineTextBox, Name: "日本語入力の設定"}, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to find header: ", err)
		}

		if err := header.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click the header: ", err)
		}

		if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to hide: ", err)
		}

		if err := optionPage.Eval(ctx,
			fmt.Sprintf(`document.getElementById('preedit_method').value = '%s';
			document.getElementById('preedit_method').dispatchEvent(new Event('change'));`, mode.name), nil); err != nil {
			s.Fatalf("Failed to update input mode to %s: %v", mode.name, err)
		}
	}

	assertInputMode(romajiInput)
	switchInputMode(kanaInput)
	assertInputMode(kanaInput)
	switchInputMode(romajiInput)
	assertInputMode(romajiInput)
}
