// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardJapaneseInputs,
		Desc:         "Checks switching between Romaji and Kana mode for Japanese inputs",
		Contacts:     []string{"myy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func VirtualKeyboardJapaneseInputs(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	vkbCtx := vkb.NewContext(cr, tconn)
	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardJapaneseInputs.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

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

	omniboxFinder := nodewith.Role(role.TextField).Attribute("inputType", "url")
	omniboxFirstResultFinder := nodewith.ClassName("OmniboxResultView").First()
	settingPageHeaderFinder := nodewith.Role(role.Heading).Name("Japanese input settings")

	const loadNewSettingDuration = 2 * time.Second

	assertInputMode := func(ctx context.Context, mode inputMode) {
		if err := uiauto.Combine(fmt.Sprintf("assert input mode is %s", mode.name),
			vkbCtx.ClickUntilVKShown(omniboxFinder),
			vkbCtx.TapKey(mode.typeKey),
			ui.Retry(5, func(ctx context.Context) error {
				omniboxFirstResultInfo, err := ui.Info(ctx, omniboxFirstResultFinder)
				if err != nil {
					return errors.Wrap(err, "failed to find omnibox results")
				}
				if !strings.Contains(omniboxFirstResultInfo.Name, mode.output) {
					return errors.Errorf("unexpected output found: got %s; want %s", omniboxFirstResultInfo.Name, mode.output)
				}
				return nil
			}),
			// Press backspace button to clear omnibox result.
			vkbCtx.TapKey("backspace"),
			ui.WaitUntilGone(omniboxFirstResultFinder),
		)(ctx); err != nil {
			s.Fatal("Failed to assert input mode: ", err)
		}
	}

	switchInputMode := func(ctx context.Context, mode inputMode) {
		if err := uiauto.Combine(fmt.Sprintf("switch input mode to %q", mode.name),
			// Click page header to deactivate virtualkeyboard.
			// Note: vkb.HideVirtualKeyboard() will not trigger reloading of setting changes.
			ui.LeftClickUntil(settingPageHeaderFinder, vkbCtx.WaitUntilHidden()),
			func(ctx context.Context) error {
				return optionPage.Eval(ctx,
					fmt.Sprintf(`document.getElementById('preedit_method').value = '%s';
					document.getElementById('preedit_method').dispatchEvent(new Event('change'));`, mode.name), nil)
			},
			// No available method to check that settings being loaded. On a low-end device, it might take a second.
			// So added sleep to wait for loading.
			ui.Sleep(loadNewSettingDuration),
		)(ctx); err != nil {
			s.Fatalf("Failed to switch input mode to %q: %v", mode.name, err)
		}
	}

	assertInputMode(ctx, romajiInput)
	switchInputMode(ctx, kanaInput)
	assertInputMode(ctx, kanaInput)
	switchInputMode(ctx, romajiInput)
	assertInputMode(ctx, romajiInput)
}
