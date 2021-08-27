// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardKoreanTyping,
		Desc:         "Checks that physical keyboard can perform basic typing in korean",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Fixture:      "chromeLoggedInForInputs",
		Timeout:      5 * time.Minute,
	})
}

func setKoreanInputType(tconn *chrome.TestConn, cr *chrome.Chrome, keyboardType string) uiauto.Action {
	return func(ctx context.Context) error {
		settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
		if err != nil {
			return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
		}
		if err := uiauto.Combine("test input method settings change",
			settings.OpenInputMethodSetting(tconn, ime.Korean),
			settings.ChangeKoreanInputMode(cr, keyboardType),
			settings.Close)(ctx); err != nil {
			return errors.Wrap(err, "failed to change IME settings")
		}
		// Ensure change has propagated to decoder - as WaitForDecoder in vkb.go
		testing.Sleep(ctx, 10.0*time.Second)
		return nil
	}
}

func PhysicalKeyboardKoreanTyping(ctx context.Context, s *testing.State) {
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
	ime.Korean.InstallAndActivate()(ctx)

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

	var subtests = []util.InputEval{
		{
			TestName: "2 set",
			InputFunc: uiauto.Combine("type in 2 set",
				setKoreanInputType(tconn, cr, "2 Set / 두벌식"),
				keyboard.TypeAction("gks")),
			ExpectedText: "한",
		},
		{
			TestName: "3 set 390 (1)",
			InputFunc: uiauto.Combine("type in 3 set 390",
				setKoreanInputType(tconn, cr, "3 Set (390) / 세벌식 (390)"),
				keyboard.TypeAction("kR")),
			ExpectedText: "걔",
		},
		{
			TestName: "3 set 390 (2)",
			InputFunc: uiauto.Combine("type in 3 set 390",
				setKoreanInputType(tconn, cr, "3 Set (390) / 세벌식 (390)"),
				keyboard.TypeAction("jfs1")),
			ExpectedText: "않",
		},
		{
			TestName: "3 set final (1)",
			InputFunc: uiauto.Combine("type in 3 set final",
				setKoreanInputType(tconn, cr, "3 Set (Final) / 세벌식 (최종)"),
				keyboard.TypeAction("kG")),
			ExpectedText: "걔",
		},
		{
			TestName: "3 set final (2)",
			InputFunc: uiauto.Combine("type in 3 set final",
				setKoreanInputType(tconn, cr, "3 Set (Final) / 세벌식 (최종)"),
				keyboard.TypeAction("ifS")),
			ExpectedText: "많",
		},
		{
			TestName: "3 set No shift (1)",
			InputFunc: uiauto.Combine("type in 3 set no shift",
				setKoreanInputType(tconn, cr, "3 Set (No Shift) / 세벌식 (순아래)"),
				keyboard.TypeAction("kR")),
			ExpectedText: "개",
		},
		{
			TestName: "3 set No shift (2)",
			InputFunc: uiauto.Combine("type in 3 set no shift",
				setKoreanInputType(tconn, cr, "3 Set (No Shift) / 세벌식 (순아래)"),
				keyboard.TypeAction("jfs1")),
			ExpectedText: "않",
		},
		{
			TestName: "Romaja",
			InputFunc: uiauto.Combine("type in 3 set no shift",
				setKoreanInputType(tconn, cr, "Romaja / 로마자"),
				keyboard.TypeAction("romaja")),
			ExpectedText: "로마자",
		},
	}

	var inputField = testserver.TextAreaInputField
	for _, subtest := range subtests {
		s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtest.TestName)

			if err := its.ValidateInputOnField(inputField, subtest.InputFunc, subtest.ExpectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate %s: %v", subtest.TestName, err)
			}
		})
	}
}
