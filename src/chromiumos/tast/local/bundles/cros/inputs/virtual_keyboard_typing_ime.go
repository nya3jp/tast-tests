// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that virtual keyboard works in different input methods",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SearchFlags:  util.IMESearchFlags(typingTestIMEs),
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      time.Duration(len(typingTestIMEs)+len(typingTestIMEsUpstream)) * time.Duration(len(typingTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               typingTestIMEs,
				Fixture:           fixture.TabletVK,
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "upstream",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               typingTestIMEsUpstream,
				Fixture:           fixture.TabletVK,
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags(typingTestIMEsUpstream),
			},
			{
				Name:              "informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val:               append(typingTestIMEs, typingTestIMEsUpstream...),
				Fixture:           fixture.TabletVK,
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags(typingTestIMEsUpstream),
			},
			{
				Name:              "lacros",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               typingTestIMEs,
				Fixture:           fixture.LacrosTabletVK,
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func VirtualKeyboardTypingIME(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)
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
