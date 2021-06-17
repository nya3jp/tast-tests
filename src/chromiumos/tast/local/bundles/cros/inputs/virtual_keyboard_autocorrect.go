// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/autocorrect"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAutocorrect,
		Desc:         "Checks that virtual keyboard can perform typing with autocorrects",
		Contacts:     []string{"tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "en_us_tablet",
				Pre:  pre.VKEnabledTablet,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_US_ENG),
					MisspeltWord:  "helol",
					CorrectWord:   "hello",
				},
			}, {
				Name: "en_us_a11y",
				Pre:  pre.VKEnabledClamshell,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_US_ENG),
					MisspeltWord:  "helol",
					CorrectWord:   "hello",
				},
			}, {
				Name: "es_es_tablet",
				Pre:  pre.VKEnabledTablet,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_ES_SPA),
					MisspeltWord:  "espanol",
					CorrectWord:   "español",
				},
			}, {
				Name: "es_es_a11y",
				Pre:  pre.VKEnabledClamshell,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_ES_SPA),
					MisspeltWord:  "espanol",
					CorrectWord:   "español",
				},
			}, {
				Name: "fr_fr_tablet",
				Pre:  pre.VKEnabledTablet,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_FR_FRA),
					MisspeltWord:  "francais",
					CorrectWord:   "français",
				},
			}, {
				Name: "fr_fr_a11y",
				Pre:  pre.VKEnabledClamshell,
				Val: autocorrect.TestCase{
					InputMethodID: string(ime.INPUTMETHOD_XKB_FR_FRA),
					MisspeltWord:  "francais",
					CorrectWord:   "français",
				},
			},
		},
	})
}

func VirtualKeyboardAutocorrect(ctx context.Context, s *testing.State) {
	testCase := s.Param().(autocorrect.TestCase)
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	imeCode := ime.IMEPrefix + testCase.InputMethodID
	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	setEnabledVKSettings := func(entryID, value string) {
		var settingsAPICall = fmt.Sprintf(
			`chrome.inputMethodPrivate.setSettings(
						 "%s", { "%s": %s})`,
			testCase.InputMethodID, entryID, value)

		tconn := s.PreValue().(pre.PreData).TestAPIConn
		if err := tconn.Eval(ctx, settingsAPICall, nil); err != nil {
			s.Fatal("Failed to set settings: ", err)
		}
	}

	setEnabledVKAutocorrectSettings := func(enabled bool) {
		var level = "0"
		if enabled {
			level = "1"
		}
		setEnabledVKSettings("virtualKeyboardAutoCorrectionLevel", level)
	}

	setEnabledVKAutocapSettings := func(enabled bool) {
		var enabledStr = "false"
		if enabled {
			enabledStr = "true"
		}
		setEnabledVKSettings("virtualKeyboardEnableCapitalization", enabledStr)
	}

	setEnabledVKAutocorrectSettings(true)
	defer setEnabledVKAutocorrectSettings(false)

	setEnabledVKAutocapSettings(false)
	defer setEnabledVKAutocapSettings(true)

	const inputField = testserver.TextAreaInputField
	if err := uiauto.Combine("validate VK autocorrect",
		its.Clear(inputField),
		its.ClickFieldUntilVKShown(inputField),
	)(ctx); err != nil {
		s.Fatal("Failed to clear then click input field to show VK: ", err)
	}

	s.Log("Wait for decoder running")
	if err := vkbCtx.WaitForDecoderEnabled(true)(ctx); err != nil {
		s.Fatal("Failed to wait for decoder running: ", err)
	}

	if err := uiauto.Combine("validate VK autocorrect",
		vkbCtx.TapKeys(strings.Split(testCase.MisspeltWord, "")),
		its.WaitForFieldValueToBe(inputField, testCase.MisspeltWord),
		vkbCtx.TapKeys([]string{"space"}),
		its.WaitForFieldValueToBe(inputField, testCase.CorrectWord+" "),
	)(ctx); err != nil {
		s.Fatal("Failed to validate VK autocorrect: ", err)
	}
}
