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
	"chromiumos/tast/local/chrome/ime"
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
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsStableModels,
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

func VirtualKeyboardJapaneseInputs(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := ime.AddAndSetInputMethod(ctx, tconn, ime.IMEPrefix+string(ime.INPUTMETHOD_NACL_MOZC_JP)); err != nil {
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

		if err := vkb.FindAndClickUntilVKShown(ctx, tconn, params); err != nil {
			s.Fatal("Failed to click the omnibox and wait for vk shown: ", err)
		}

		if err := vkb.TapKey(ctx, tconn, mode.typeKey); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}

		omniboxResultParams := ui.FindParams{ClassName: "OmniboxResultView"}

		// Value change can be a bit delayed after input.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			searchResultFirstNode, err := ui.FindWithTimeout(ctx, tconn, omniboxResultParams, 2*time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to find omnibox results")
			}
			defer searchResultFirstNode.Release(ctx)

			if !strings.Contains(searchResultFirstNode.Name, mode.output) {
				return errors.Errorf("unexpected output found: got %s; want %s", searchResultFirstNode.Name, mode.output)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to input with virtual keyboard: ", err)
		}

		// Delete input in omnibox.
		if err := vkb.TapKey(ctx, tconn, "backspace"); err != nil {
			s.Fatal("Failed to delete with virtual keyboard: ", err)
		}

		if err := ui.WaitUntilGone(ctx, tconn, omniboxResultParams, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for omnibox search result disappear after deleting input: ", err)
		}
	}

	switchInputMode := func(mode inputMode) {
		s.Log("Switching input mode to ", mode.name)

		// Click page header to deactive virtualkeyboard.
		// Note: vkb.HideVirtualKeyboard() will not trigger reloading of setting changes.
		pageHeader := "Japanese input settings"
		header, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeHeading, Name: pageHeader}, 20*time.Second)
		if err != nil {
			s.Fatalf("Failed to find header %s: %v", pageHeader, err)
		}

		condition := func(ctx context.Context) (bool, error) {
			isVKShown, err := vkb.IsShown(ctx, tconn)
			return !isVKShown, err
		}
		opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 2 * time.Second}
		if err := header.LeftClickUntil(ctx, condition, &opts); err != nil {
			s.Fatal("Failed to click header until vk hidden: ", err)
		}

		if err := optionPage.Eval(ctx,
			fmt.Sprintf(`document.getElementById('preedit_method').value = '%s';
			document.getElementById('preedit_method').dispatchEvent(new Event('change'));`, mode.name), nil); err != nil {
			s.Fatalf("Failed to update input mode to %s: %v", mode.name, err)
		}

		// No available method to check that settings being loaded. On a low-end device, it might take a second.
		// So added 2 seconds sleep to wait for loading.
		const loadNewSettingDuration = 2 * time.Second

		s.Log("Warmup: Waiting for loading new settings")
		if err := testing.Sleep(ctx, loadNewSettingDuration); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
	}

	assertInputMode(romajiInput)
	switchInputMode(kanaInput)
	assertInputMode(kanaInput)
	switchInputMode(romajiInput)
	assertInputMode(romajiInput)
}
