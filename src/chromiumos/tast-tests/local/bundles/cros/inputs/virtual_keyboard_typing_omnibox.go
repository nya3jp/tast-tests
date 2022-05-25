// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast-tests/local/apps"
	"chromiumos/tast-tests/local/bundles/cros/inputs/data"
	"chromiumos/tast-tests/local/bundles/cros/inputs/fixture"
	"chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"chromiumos/tast-tests/local/bundles/cros/inputs/util"
	"chromiumos/tast-tests/local/chrome/ash"
	"chromiumos/tast-tests/local/chrome/browser"
	"chromiumos/tast-tests/local/chrome/ime"
	"chromiumos/tast-tests/local/chrome/uiauto"
	"chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"chromiumos/tast-tests/local/chrome/uiauto/role"
	"chromiumos/tast-tests/local/chrome/uiauto/vkb"
	"chromiumos/tast-tests/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingOmnibox,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "guest",
				Fixture:           fixture.TabletVKInGuest,
				Val:               []ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "a11y",
				Fixture:           fixture.ClamshellVK,
				Val:               []ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream", "informational"},
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin},
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:              "guest_lacros",
				Fixture:           fixture.LacrosTabletVKInGuest,
				Val:               []ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros"},
			},
			{
				Name:              "a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				Val:               []ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	uc.SetTestName(s.TestName())

	vkbCtx := vkb.NewContext(cr, tconn)

	testIMEs := s.Param().([]ime.InputMethod)

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

			br := apps.Chrome
			if s.FixtValue().(fixture.FixtData).BrowserType == browser.TypeLacros {
				br = apps.Lacros
			}
			// Warning: Please do not launch Browser via cr.NewConn(ctx, "")
			// to test omnibox typing. It might be indeterminate whether default url string
			// "about:blank" is highlighted or not.
			// In that case, typing test can either replace existing url or insert into it.
			// A better way to do it is launching Browser from launcher, url is empty by default.
			if err := apps.Launch(ctx, tconn, br.ID); err != nil {
				s.Fatalf("Failed to launch %s: %s", apps.Chrome.Name, err)
			}
			defer func(ctx context.Context) {
				if err := apps.Close(ctx, tconn, br.ID); err != nil {
					testing.ContextLog(ctx, "Failed to close Chrome browser: ", err)
				}
			}(cleanupCtx)

			if err := ash.WaitForApp(ctx, tconn, br.ID, time.Minute); err != nil {
				s.Fatalf("%s did not appear in shelf after launch: %s", br.Name, err)
			}

			omniboxFinder := nodewith.Role(role.TextField).Attribute("inputType", "url")

			validateAction := uiauto.Combine("verify virtual keyboard input on omnibox",
				vkbCtx.ClickUntilVKShown(omniboxFinder),
				vkbCtx.TapKeysIgnoringCase(inputData.CharacterKeySeq),
				func(ctx context.Context) error {
					if inputData.SubmitFromSuggestion {
						return vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx)
					}
					return nil
				},
				// Validate text.
				util.WaitForFieldTextToBeIgnoringCase(tconn, omniboxFinder, inputData.ExpectedText),
			)

			if err := uiauto.UserAction("VK typing input",
				validateAction,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField: "Omnibox",
						useractions.AttributeFeature:    useractions.FeatureVKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to verify virtual keyboard input on omnibox: ", err)
			}
		}
	}
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, testIMEs, []data.Message{data.TypingMessageHello}, subtest)
}
