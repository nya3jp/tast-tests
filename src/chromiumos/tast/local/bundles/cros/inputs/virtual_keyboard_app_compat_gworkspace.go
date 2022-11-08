// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// vkGoogleWorkspaceApptestCase struct encapsulates parameters for each test.
type vkGoogleWorkspaceApptestCase struct {
	feature string // input method feaure to test
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAppCompatGworkspace,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test virtual keyboard for google workspace",
		Contacts:     []string{"xiuwen@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance, ime.EnglishUS}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "accentkey_doc",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "accent_key",
				},
			},
			{
				Name:              "accentkey_slide",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "accent_key",
				},
			},
			{
				Name:              "accentkey_sheet",
				Fixture:           fixture.GoogleSheets,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "accent_key",
				},
			},
			{
				Name:              "typing_doc",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "typing",
				},
			},
			{
				Name:              "typing_slide",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "typing",
				},
			},
			{
				Name:              "glide_typing_doc",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "glide_typing",
				},
			},
			{
				Name:              "glide_typing_slide",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: vkGoogleWorkspaceApptestCase{
					feature: "glide_typing",
				},
			},
		},
	})
}

func VirtualKeyboardAppCompatGworkspace(ctx context.Context, s *testing.State) {
	testCase := s.Param().(vkGoogleWorkspaceApptestCase)
	cr := s.FixtValue().(fixture.GworkspaceFixtData).Chrome
	tconn := s.FixtValue().(fixture.GworkspaceFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.GworkspaceFixtData).UserContext
	// vkbCtx := s.FixtValue().(fixture.GworkspaceFixtData).VirtualKeyboardContext
	vkbCtx := vkb.NewContext(cr, tconn)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	ui := uiauto.New(tconn)

	switch testCase.feature {
	case "accent_key":
		var subtests = []util.AppCompatTestCase{
			{
				TestName:      "french accent key é",
				LanguageLabel: "FR",
				InputFunc:     util.TypingAccentKeyAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "FR", "é", "e"),
				InputMethod:   ime.FrenchFrance,
			},
			{
				TestName:      "french accent key ç",
				LanguageLabel: "FR",
				InputFunc:     util.TypingAccentKeyAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "FR", "ç", "c"),
				InputMethod:   ime.FrenchFrance,
			},
			{
				TestName:      "french accent key ö",
				LanguageLabel: "FR",
				InputFunc:     util.TypingAccentKeyAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "FR", "ö", "o"),
				InputMethod:   ime.FrenchFrance,
			},
		}

		for _, subtest := range subtests {
			if err := util.InstallIME(ctx, uc, subtest.InputMethod); err != nil {
				s.Fatal("Fail to set input method: ", err)
			}

			s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
				if err := uiauto.UserAction(
					subtest.TestName,
					subtest.InputFunc,
					uc, &useractions.UserActionCfg{
						Attributes: map[string]string{
							useractions.AttributeTestScenario: subtest.TestName,
							useractions.AttributeFeature:      useractions.FeatureVKTyping,
						},
					},
				)(ctx); err != nil {
					s.Fatalf("Failed to validate typing in test %s: %v", subtest.TestName, err)
				}
			})
		}
	case "typing":
		var subtests = []util.AppCompatTestCase{
			{
				TestName:      "french vk typing",
				LanguageLabel: "FR",
				InputFunc:     util.TypingLettersAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "FR", "frtyping"),
				InputMethod:   ime.FrenchFrance,
			},
			{
				TestName:      "EnglishUS vk typing",
				LanguageLabel: "US",
				InputFunc:     util.TypingLettersAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "US", "entyping"),
				InputMethod:   ime.EnglishUS,
			},
		}

		for _, subtest := range subtests {
			if err := util.InstallIME(ctx, uc, subtest.InputMethod); err != nil {
				s.Fatal("Fail to set input method: ", err)
			}

			s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
				if err := uiauto.UserAction(
					subtest.TestName,
					subtest.InputFunc,
					uc, &useractions.UserActionCfg{
						Attributes: map[string]string{
							useractions.AttributeTestScenario: subtest.TestName,
							useractions.AttributeFeature:      useractions.FeatureVKTyping,
						},
					},
				)(ctx); err != nil {
					s.Fatalf("Failed to validate typing in test %s: %v", subtest.TestName, err)
				}
			})
			util.ClickEnterToStartNewLine(ctx)
		}

	case "glide_typing":
		var subtests = []util.AppCompatTestCase{
			{
				TestName:      "french vk gliding type",
				LanguageLabel: "FR",
				InputFunc:     util.GlideTypingAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "FR", "bonjour"),
				InputMethod:   ime.FrenchFrance,
			},
			{
				TestName:      "EnglishUS vk gliding type",
				LanguageLabel: "US",
				InputFunc:     util.GlideTypingAccordingToLanguageOnVK(ctx, tconn, ui, vkbCtx, "US", "hello"),
				InputMethod:   ime.EnglishUS,
			},
		}
		for _, subtest := range subtests {
			if err := util.InstallIME(ctx, uc, subtest.InputMethod); err != nil {
				s.Fatal("Fail to set input method: ", err)
			}

			s.Run(ctx, subtest.TestName, func(ctx context.Context, s *testing.State) {
				if err := uiauto.UserAction(
					subtest.TestName,
					subtest.InputFunc,
					uc, &useractions.UserActionCfg{
						Attributes: map[string]string{
							useractions.AttributeTestScenario: subtest.TestName,
							useractions.AttributeFeature:      useractions.FeatureVKTyping,
						},
					},
				)(ctx); err != nil {
					s.Fatalf("Failed to validate typing in test %s: %v", subtest.TestName, err)
				}
			})
			util.ClickEnterToStartNewLine(ctx)
		}
	}
}
