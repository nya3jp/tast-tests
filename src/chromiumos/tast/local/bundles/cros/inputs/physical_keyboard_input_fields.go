// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardInputFields,
		Desc:         "Checks that physical keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Fixture:      "chromeLoggedInForInputs",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "us_en",
				Val:  ime.INPUTMETHOD_XKB_US_ENG,
			},
			{
				Name: "jp_ja",
				Val:  ime.INPUTMETHOD_NACL_MOZC_US,
			},
			{
				Name: "pinyin",
				Val:  ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
			},
		},
	})
}

func PhysicalKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	imeCode := ime.IMEPrefix + string(s.Param().(ime.InputMethodCode))

	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	vkbCtx := vkb.NewContext(cr, tconn)

	var subtests []util.FieldInputEval

	switch s.Param().(ime.InputMethodCode) {
	case ime.INPUTMETHOD_XKB_US_ENG:
		subtests = []util.FieldInputEval{
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    keyboard.TypeAction(`1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`),
				ExpectedText: `1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`,
			}, {
				InputField:   testserver.TextInputField,
				InputFunc:    keyboard.TypeAction("qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"),
				ExpectedText: "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM",
			},
		}
		break
	case ime.INPUTMETHOD_NACL_MOZC_US:
		subtests = []util.FieldInputEval{
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select default candidate", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("nihongo "), keyboard.AccelAction("Enter")),
				ExpectedText: "日本語",
			},
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select candidate by space", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("ni  "), keyboard.AccelAction("Enter")),
				ExpectedText: "２",
			},
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select candidate by number", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("ni  5"), keyboard.AccelAction("Enter")),
				ExpectedText: "二",
			},
		}
	case ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED:
		subtests = []util.FieldInputEval{
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select default candidate", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("zhongwen ")),
				ExpectedText: "中文",
			},
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select candidate by down arrow", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("zhong"), keyboard.AccelAction("Down"), keyboard.AccelAction("Space")),
				ExpectedText: "种",
			},
			{
				InputField:   testserver.TextAreaInputField,
				InputFunc:    uiauto.Combine("type and select candidate by number", vkbCtx.WaitForDecoderEnabled(true), keyboard.TypeAction("zhong2")),
				ExpectedText: "中",
			},
		}
		break
	default:
		s.Fatalf("%s is not supported", imeCode)
	}

	for _, subtest := range subtests {
		s.Run(ctx, string(subtest.InputField), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.InputField))
			inputField := subtest.InputField

			if err := its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
