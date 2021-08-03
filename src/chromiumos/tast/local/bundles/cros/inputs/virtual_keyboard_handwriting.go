// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Documentation on file format can be found in go/tast-handwriting-svg-parsing.
const (
	handwritingWarmupFile  = "handwriting_digit_3.svg"
	handwritingWarmupDigit = "3"
)

var hwTestMessages = []data.Message{data.HandwritingMessageHello}
var hwTestIMEs = []ime.InputMethod{
	ime.Japanese,
	ime.ChinesePinyin,
	ime.EnglishUS,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Data:         append(data.ExtractExternalFiles(hwTestMessages, hwTestIMEs), handwritingWarmupFile),
		Timeout:      time.Duration(len(hwTestIMEs)) * time.Duration(len(hwTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name:              "docked",
				Val:               false, // false for docked-mode VK.
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "docked_informational",
				Val:               false, // false for docked-mode VK.
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "floating",
				Val:               true, // true for floating-mode VK.
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "floating_informational",
				Val:               true, // true for floating-mode VK.
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	isFloating := s.Param().(bool)
	// Options containing preconditions.
	opts := []chrome.Option{
		chrome.VKEnabled(),
		chrome.ExtraArgs("--force-tablet-mode=touch_view"),
	}

	// Add precondition of requiring a floating keyboard if testing for floating handwriting input.
	if isFloating {
		opts = append(opts, chrome.EnableFeatures("VirtualKeyboardFloatingDefault"))
	}

	// TODO(crbug/1173252): Clean up states within Chrome using preconditions.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to connect to new Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	// Creates subtest that runs the test logic using inputData.
	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to show VK: ", err)
			}

			// Switch to handwriting layout.
			hwCtx, err := vkbCtx.SwitchToHandwritingAndCloseInfoDialogue(ctx)
			if err != nil {
				s.Fatal("Failed to switch to handwriting: ", err)
			}

			// Warm-up steps to check handwriting engine ready.
			checkEngineReady := uiauto.Combine("Wait for handwriting engine to be ready",
				its.Clear(inputField),
				hwCtx.DrawStrokesFromFile(s.DataPath(handwritingWarmupFile)),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), handwritingWarmupDigit),
				hwCtx.ClearHandwritingCanvas(),
				its.Clear(inputField))

			if err := uiauto.Combine("Test handwriting on virtual keyboard",
				hwCtx.WaitForHandwritingEngineReady(checkEngineReady),
				hwCtx.DrawStrokesFromFile(s.DataPath(inputData.HandwritingFile)),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), inputData.ExpectedText),
			)(ctx); err != nil {
				s.Fatal("Failed to verify handwriting input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, hwTestIMEs, hwTestMessages, subtest)
}
