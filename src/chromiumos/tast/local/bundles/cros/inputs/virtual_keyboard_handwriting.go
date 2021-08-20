// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var hwTestMessages = []data.Message{data.HandwritingMessageHello}
var hwTestIMEs = []ime.InputMethod{
	ime.AlphanumericWithJapaneseKeyboard,
	ime.Arabic,
	ime.ChinesePinyin,
	ime.EnglishUK,
	ime.EnglishUS,
	ime.EnglishUSWithInternationalKeyboard,
	ime.Japanese,
	ime.Korean,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools", "group:input-tools-upstream"},
		Data:         data.ExtractExternalFiles(hwTestMessages, hwTestIMEs),
		Pre:          pre.VKEnabledReset,
		Timeout:      time.Duration(len(hwTestIMEs)) * time.Duration(len(hwTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name:              "docked",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "docked_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "floating",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "floating_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
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

	// Switch to floating mode if needed.
	isFloating := strings.Contains(s.TestName(), "floating")
	if isFloating {
		if err := uiauto.Combine("validate handwriting input",
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.SetFloatingMode(true),
		)(ctx); err != nil {
			s.Fatal("Failed to switch to floating mode: ", err)
		}

		defer func(ctx context.Context) {
			if err := uiauto.Combine("switch back to docked mode and hide VK",
				its.ClickFieldUntilVKShown(inputField),
				vkbCtx.SetFloatingMode(false),
				vkbCtx.HideVirtualKeyboard(),
			)(ctx); err != nil {
				s.Log("Failed to cleanup floating mode: ", err)
			}
		}(cleanupCtx)
	}

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

			if err := its.ValidateInputFieldForMode(inputField, util.InputWithHandWriting, inputData, s.DataPath)(ctx); err != nil {
				s.Fatal("Failed to validate handwriting input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, hwTestIMEs, hwTestMessages, subtest)
}
