// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
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
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that Transliteration physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
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
			},
			{
				Name: "gu",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Gujarati,
					typingKeys:          "gujarati",
					expectedComposition: "ગુજરાતી",
				},
			},
			{
				Name: "hi",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Hindi,
					typingKeys:          "hindee",
					expectedComposition: "हिंदी",
				},
			},
			{
				Name: "kn",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Kannada,
					typingKeys:          "kannada",
					expectedComposition: "ಕನ್ನಡ",
				},
			},
			{
				Name: "ml",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Malayalam,
					typingKeys:          "malayalam",
					expectedComposition: "മലയാളം",
				},
			},
			{
				Name: "mr",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Marathi,
					typingKeys:          "marathi",
					expectedComposition: "मराठी",
				},
			},
			{
				Name: "ne",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.NepaliTransliteration,
					typingKeys:          "nepali",
					expectedComposition: "नेपाली",
				},
			},
			{
				Name: "or",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Odia,
					typingKeys:          "odia",
					expectedComposition: "ଓଡ଼ିଆ",
				},
			},
			{
				Name: "fa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.PersianTransliteration,
					typingKeys:          "farsi",
					expectedComposition: "فارسی",
				},
			},
			{
				Name: "pa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Punjabi,
					typingKeys:          "pajabi",
					expectedComposition: "ਪੰਜਾਬੀ",
				},
			},
			{
				Name: "sa",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Sanskrit,
					typingKeys:          "samskrtam",
					expectedComposition: "संस्कृतम्",
				},
			},
			{
				Name: "ta",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Tamil,
					typingKeys:          "tamil",
					expectedComposition: "தமிழ்",
				},
			},
			{
				Name: "te",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Telugu,
					typingKeys:          "telugu",
					expectedComposition: "తెలుగు",
				},
			},
			{
				Name: "ur",
				Val: pkTransliterationTestCase{
					inputMethod:         ime.Urdu,
					typingKeys:          "urdu",
					expectedComposition: "اردو",
				},
			},
		},
	})
}

func PhysicalKeyboardTransliterationTyping(ctx context.Context, s *testing.State) {
	testCase := s.Param().(pkTransliterationTestCase)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputMethod := testCase.inputMethod
	if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
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
