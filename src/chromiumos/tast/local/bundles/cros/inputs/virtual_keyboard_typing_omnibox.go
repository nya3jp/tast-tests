// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingOmnibox,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin}),
			},
			{
				Name:              "guest",
				Fixture:           fixture.TabletVKInGuest,
				Val:               []ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean}),
			},
			{
				Name:              "a11y",
				Fixture:           fixture.ClamshellVK,
				Val:               []ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic}),
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.Arabic}),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin}),
			},
			{
				Name:              "guest_lacros",
				Fixture:           fixture.LacrosTabletVKInGuest,
				Val:               []ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean}),
			},
			{
				Name:              "a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				Val:               []ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic}),
			},
		},
	})
}

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

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

			br, err := apps.PrimaryBrowser(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get browser app: ", err)
			}
			// Warning: Please do not launch Browser via cr.NewConn(ctx, "")
			// to test omnibox typing. It might be indeterminate whether default url string
			// "about:blank" is highlighted or not.
			// In that case, typing test can either replace existing url or insert into it.
			// A better way to do it is launching Browser from launcher, url is empty by default.
			if err := apps.Launch(ctx, tconn, br.ID); err != nil {
				s.Fatalf("Failed to launch %s: %s", br.Name, err)
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
