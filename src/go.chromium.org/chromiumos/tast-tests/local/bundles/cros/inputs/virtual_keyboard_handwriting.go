// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/data"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/fixture"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/testserver"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/util"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/ime"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/vkb"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		Data:         data.ExtractExternalFiles(hwTestMessages, append(hwTestIMEs, hwTestIMEsUpstream...)),
		Timeout:      2 * time.Duration(len(hwTestIMEs)+len(hwTestIMEsUpstream)) * time.Duration(len(hwTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name:              "docked",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val:               hwTestIMEs,
			},
			{
				Name:              "docked_upstream",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream", "informational"},
				Val:               hwTestIMEsUpstream,
			},
			{
				Name:              "docked_informational",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
			{
				Name:              "floating",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Val:               hwTestIMEs,
			},
			{
				Name:              "floating_upstream",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				Val:               hwTestIMEsUpstream,
			},
			{
				Name:              "floating_informational",
				Fixture:           fixture.AnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
			{
				Name:              "docked_lacros",
				Fixture:           fixture.LacrosAnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
			{
				Name:              "floating_lacros",
				Fixture:           fixture.LacrosAnyVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
				Val:               append(hwTestIMEs, hwTestIMEsUpstream...),
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	uc.SetTestName(s.TestName())

	testIMEs := s.Param().([]ime.InputMethod)

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launch inputs test web server.
	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

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
