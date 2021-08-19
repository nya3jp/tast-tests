// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type glideTypingTestPram struct {
	floatLayout bool
	inputMethod ime.InputMethod
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardGlideTyping,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(3 * time.Minute),
		Params: []testing.Param{
			{
				Name: "tablet_docked",
				Pre:  pre.VKEnabledTabletReset,
				Val: glideTypingTestPram{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
			}, {
				Name: "tablet_float",
				Pre:  pre.VKEnabledTabletReset,
				Val: glideTypingTestPram{
					floatLayout: true,
					inputMethod: ime.EnglishUSWithInternationalKeyboard,
				},
			}, {
				Name: "clamshell_a11y_docked",
				Pre:  pre.VKEnabledClamshellReset,
				Val: glideTypingTestPram{
					floatLayout: false,
					inputMethod: ime.Swedish,
				},
			},
			{
				Name: "clamshell_a11y_float",
				Pre:  pre.VKEnabledClamshellReset,
				Val: glideTypingTestPram{
					floatLayout: true,
					inputMethod: ime.SpanishSpain,
				},
			},
		},
	})
}

func VirtualKeyboardGlideTyping(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := s.Param().(glideTypingTestPram).inputMethod
	shouldFloatLayout := s.Param().(glideTypingTestPram).floatLayout

	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
		s.Fatalf("Faield to install and activate input method %q: %v", inputMethod, err)
	}

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	vkbCtx := vkb.NewContext(cr, tconn)

	glideTypingWord, ok := data.TypingMessageHello.GetInputData(inputMethod)
	if !ok {
		s.Fatalf("Test Data for input method %v does not exist", inputMethod)
	}
	keySeq := glideTypingWord.CharacterKeySeq

	// Wrap a function to set float VK only once during the test.
	var isFloatLayout bool
	setFloatVK := func(ctx context.Context) error {
		if !isFloatLayout && shouldFloatLayout {
			isFloatLayout = true
			return vkbCtx.SetFloatingMode(true)(ctx)
		}
		return nil
	}

	setGlideTyping := func(isEnabled bool) uiauto.Action {
		return func(ctx context.Context) error {
			setting, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
			if err != nil {
				return errors.Wrap(err, "failed to launch input settings")
			}
			return uiauto.Combine("change glide typing setting",
				setting.OpenInputMethodSetting(tconn, inputMethod),
				setting.ToggleGlideTyping(cr, tconn, isEnabled),
				setting.CloseAction(),
			)(ctx)
		}
	}

	validateGlideTyping := func(inputField testserver.InputField, isGlideTypingEnabled bool) uiauto.Action {
		return uiauto.Combine("validate glide typing",
			vkbCtx.HideVirtualKeyboard(),
			its.Clear(inputField),
			its.ClickFieldUntilVKShown(inputField),
			setFloatVK,
			util.GlideTyping(tconn, keySeq),
			// Check if input field text exactly matches glide typing result.
			// Select candidate from suggestion bar is also acceptable.
			func(ctx context.Context) error {
				if !isGlideTypingEnabled {
					// Should submit the last key if glide typing disabled.
					return its.ValidateResult(inputField, keySeq[len(keySeq)-1])(ctx)
				}
				if err := util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), glideTypingWord.ExpectedText)(ctx); err != nil {
					s.Log("Input field text does not match glide typing gesture")
					s.Log("Check if it is in suggestion bar")
					return uiauto.Combine("selecting candidate from suggestion bar",
						vkbCtx.SelectFromSuggestionIgnoringCase(glideTypingWord.ExpectedText),
						its.ValidateResult(inputField, glideTypingWord.ExpectedText),
					)(ctx)
				}
				return nil
			},
		)
	}

	// Test glide typing is enabled by default.
	testName := "default"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		// Test default behavior in Textarea field.
		inputField := testserver.TextAreaInputField
		if err := validateGlideTyping(inputField, true)(ctx); err != nil {
			s.Fatal("Failed to validate glide typing: ", err)
		}
	})

	// Test glide typing is disabled in not-applicable field.
	testName = "not_applicable"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		// Test not-applicable behavior in password field.
		inputField := testserver.PasswordInputField
		if err := validateGlideTyping(inputField, false)(ctx); err != nil {
			s.Fatal("Failed to validate glide typing: ", err)
		}
	})

	// Test glide typing can be disabled in settings.
	testName = "disable"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		// Test disabled behavior in Textarea field.
		inputField := testserver.TextAreaInputField
		if err := uiauto.Combine("disable glide typing and verify",
			setGlideTyping(false),
			validateGlideTyping(inputField, false),
		)(ctx); err != nil {
			s.Fatal("Failed to verfy glide typing disabled: ", err)
		}
	})

	// Test glide typing can be re-enabled in settings.
	testName = "re-enable"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		// Test re-enabled behavior in Textarea field.
		inputField := testserver.TextAreaInputField
		if err := uiauto.Combine("disable glide typing and verify",
			setGlideTyping(true),
			validateGlideTyping(inputField, true),
		)(ctx); err != nil {
			s.Fatal("Failed to verfy glide typing re-enabled: ", err)
		}
	})
}
