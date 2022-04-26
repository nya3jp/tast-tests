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
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var hwTestMessages = []data.Message{data.HandwritingMessageHello}
var hwTestIMEs = []ime.InputMethod{
	ime.AlphanumericWithJapaneseKeyboard,
	ime.ChinesePinyin,
	ime.EnglishUK,
	ime.EnglishUS,
	ime.EnglishUSWithInternationalKeyboard,
	ime.Japanese,
	ime.Korean,
}

var hwTestIMEsUpstream = []ime.InputMethod{
	// TODO(b/230424689): Add Arabic to CQ once issue fixed.
	ime.Arabic,
	ime.EnglishSouthAfrica,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		Data:         data.ExtractExternalFiles(hwTestMessages, append(hwTestIMEs, hwTestIMEsUpstream...)),
		Timeout:      2 * time.Duration(len(hwTestIMEs)+len(hwTestIMEsUpstream)) * time.Duration(len(hwTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name:              "docked",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val:               hwTestIMEs,
			},
			{
				Name:              "docked_upstream",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream", "informational"},
				Val:               hwTestIMEsUpstream,
			},
			{
				Name:              "docked_informational",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
			{
				Name:              "floating",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val:               hwTestIMEs,
			},
			{
				Name:              "floating_upstream",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				Val:               hwTestIMEsUpstream,
			},
			{
				Name:              "floating_informational",
				Pre:               pre.VKEnabledReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
			{
				Name:              "docked_fixture",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				Val:               hwTestIMEs,
			},
			{
				Name:              "floating_fixture",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				Val:               hwTestIMEs,
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	testIMEs := s.Param().([]ime.InputMethod)

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
			vkbCtx.SetFloatingMode(uc, true),
		)(ctx); err != nil {
			s.Fatal("Failed to switch to floating mode: ", err)
		}

		defer func(ctx context.Context) {
			if err := uiauto.Combine("switch back to docked mode and hide VK",
				its.ClickFieldUntilVKShown(inputField),
				vkbCtx.SetFloatingMode(uc, false),
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

			if err := its.ValidateInputFieldForMode(uc, inputField, util.InputWithHandWriting, inputData, s.DataPath)(ctx); err != nil {
				s.Fatal("Failed to validate handwriting input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, testIMEs, hwTestMessages, subtest)
}
