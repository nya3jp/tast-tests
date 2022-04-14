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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var typingTestIMEs = []ime.InputMethod{
	ime.EnglishUS,
	ime.JapaneseWithUSKeyboard,
	ime.ChinesePinyin,
	ime.EnglishUSWithInternationalKeyboard,
	ime.EnglishUK,
	ime.SpanishSpain,
	ime.Swedish,
	ime.EnglishCanada,
	ime.AlphanumericWithJapaneseKeyboard,
	ime.Japanese,
	ime.FrenchFrance,
	ime.Cantonese,
	ime.ChineseCangjie,
	ime.Korean,
	ime.Arabic,
}

var typingTestIMEsUpstream = []ime.InputMethod{
	ime.EnglishSouthAfrica,
}

var typingTestMessages = []data.Message{data.TypingMessageHello}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingIME,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that virtual keyboard works in different input methods",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(typingTestIMEs)+len(typingTestIMEsUpstream)) * time.Duration(len(typingTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Pre:               pre.VKEnabledTabletReset,
				Val:               typingTestIMEs,
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "upstream",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Pre:               pre.VKEnabledTabletReset,
				Val:               typingTestIMEsUpstream,
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Pre:               pre.VKEnabledTabletReset,
				Val:               append(typingTestIMEs, typingTestIMEsUpstream...),
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "fixture",
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               typingTestIMEs,
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardTypingIME(ctx context.Context, s *testing.State) {
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

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()
	inputField := testserver.TextAreaNoCorrectionInputField
	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ValidateInputFieldForMode(uc, inputField, util.InputWithVK, inputData, s.DataPath)(ctx); err != nil {
				s.Fatal("Failed to validate virtual keyboard input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, typingTestIMEs, typingTestMessages, subtest)
}
