// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// pkTransliterationTestCase struct encapsulates parameters for each transliteration test.
type pkTransliterationTestCase struct {
	inputMethod         ime.InputMethod
	typingKeys          string
	expectedComposition string
}

func init() {
	// TODO(b/213799105): Add 'group:input-tools-upstream' once system PK transliteration is enabled by default.
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardTransliterationTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Transliteration physical keyboard works",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "el",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.GreekTransliteration,
					typingKeys:          "ellinika",
					expectedComposition: "Ελληνικά",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.GreekTransliteration}),
			},
			{
				Name: "gu",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Gujarati,
					typingKeys:          "gujarati",
					expectedComposition: "ગુજરાતી",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Gujarati}),
			},
			{
				Name: "hi",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Hindi,
					typingKeys:          "hindee",
					expectedComposition: "हिंदी",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Hindi}),
			},
			{
				Name: "kn",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Kannada,
					typingKeys:          "kannada",
					expectedComposition: "ಕನ್ನಡ",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Kannada}),
			},
			{
				Name: "ml",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Malayalam,
					typingKeys:          "malayalam",
					expectedComposition: "മലയാളം",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Malayalam}),
			},
			{
				Name: "mr",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Marathi,
					typingKeys:          "marathi",
					expectedComposition: "मराठी",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Marathi}),
			},
			{
				Name: "ne",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.NepaliTransliteration,
					typingKeys:          "nepali",
					expectedComposition: "नेपाली",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.NepaliTransliteration}),
			},
			{
				Name: "or",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Odia,
					typingKeys:          "odia",
					expectedComposition: "ଓଡ଼ିଆ",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Odia}),
			},
			{
				Name: "fa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.PersianTransliteration,
					typingKeys:          "farsi",
					expectedComposition: "فارسی",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.PersianTransliteration}),
			},
			{
				Name: "pa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Punjabi,
					typingKeys:          "pajabi",
					expectedComposition: "ਪੰਜਾਬੀ",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Punjabi}),
			},
			{
				Name: "sa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Sanskrit,
					typingKeys:          "samskrtam",
					expectedComposition: "संस्कृतम्",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Sanskrit}),
			},
			{
				Name: "ta",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Tamil,
					typingKeys:          "tamil",
					expectedComposition: "தமிழ்",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Tamil}),
			},
			{
				Name: "te",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Telugu,
					typingKeys:          "telugu",
					expectedComposition: "తెలుగు",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Telugu}),
			},
			{
				Name: "ur",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Urdu,
					typingKeys:          "urdu",
					expectedComposition: "اردو",
				},
				Fixture:          fixture.ClamshellNonVK,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Urdu}),
			},
			// ------lacros variants below---------------
			{
				Name: "el_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.GreekTransliteration,
					typingKeys:          "ellinika",
					expectedComposition: "Ελληνικά",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.GreekTransliteration}),
			},
			{
				Name: "gu_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Gujarati,
					typingKeys:          "gujarati",
					expectedComposition: "ગુજરાતી",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Gujarati}),
			},
			{
				Name: "hi_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Hindi,
					typingKeys:          "hindee",
					expectedComposition: "हिंदी",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Hindi}),
			},
			{
				Name: "kn_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Kannada,
					typingKeys:          "kannada",
					expectedComposition: "ಕನ್ನಡ",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Kannada}),
			},
			{
				Name: "ml_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Malayalam,
					typingKeys:          "malayalam",
					expectedComposition: "മലയാളം",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Malayalam}),
			},
			{
				Name: "mr_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Marathi,
					typingKeys:          "marathi",
					expectedComposition: "मराठी",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Marathi}),
			},
			{
				Name: "ne_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.NepaliTransliteration,
					typingKeys:          "nepali",
					expectedComposition: "नेपाली",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.NepaliTransliteration}),
			},
			{
				Name: "or_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Odia,
					typingKeys:          "odia",
					expectedComposition: "ଓଡ଼ିଆ",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Odia}),
			},
			{
				Name: "fa_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.PersianTransliteration,
					typingKeys:          "farsi",
					expectedComposition: "فارسی",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.PersianTransliteration}),
			},
			{
				Name: "pa_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Punjabi,
					typingKeys:          "pajabi",
					expectedComposition: "ਪੰਜਾਬੀ",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Punjabi}),
			},
			{
				Name: "sa_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Sanskrit,
					typingKeys:          "samskrtam",
					expectedComposition: "संस्कृतम्",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Sanskrit}),
			},
			{
				Name: "ta_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Tamil,
					typingKeys:          "tamil",
					expectedComposition: "தமிழ்",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Tamil}),
			},
			{
				Name: "te_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Telugu,
					typingKeys:          "telugu",
					expectedComposition: "తెలుగు",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Telugu}),
			},
			{
				Name: "ur_lacros",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Urdu,
					typingKeys:          "urdu",
					expectedComposition: "اردو",
				},
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Urdu}),
			},
		},
	})
}

func PhysicalKeyboardTransliterationTyping(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkTransliterationTestCase)

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	inputField := testserver.TextAreaInputField

	validateAction := uiauto.Combine("type then verify composition and candidates",
		its.ValidateInputOnField(inputField, kb.TypeAction(testCase.typingKeys), testCase.expectedComposition),
		util.GetNthCandidateTextAndThen(tconn, 0, func(candidate string) uiauto.Action {
			return util.WaitForFieldTextToBe(tconn, inputField.Finder(), candidate)
		}),
	)

	if err := uiauto.UserAction(
		"Transliteration PK input",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf(`type %q to get %q`, testCase.typingKeys, testCase.expectedComposition),
				useractions.AttributeFeature:      useractions.FeaturePKTyping,
				useractions.AttributeInputField:   string(inputField),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify transliteration typing: ", err)
	}
}
