// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardTransliterationTyping,
		Desc:         "Checks that Transliteration physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream", "informational"},
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
	cr := s.PreValue().(pre.PreData).Chrome

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	testCase := s.Param().(pkTransliterationTestCase)
	imeCode := ime.ChromeIMEPrefix + testCase.inputMethod.ID

	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField

	subtests := []struct {
		Name   string
		Action uiauto.Action
	}{
		{
			// Type something and check that the composition is correct and matches the top candidate.
			Name: "TypingSetsCompositionToCorrectTopCandidate",
			Action: uiauto.Combine("type then verify composition and candidates",
				its.ValidateInputOnField(inputField, kb.TypeAction(testCase.typingKeys), testCase.expectedComposition),
				util.GetNthCandidateTextAndThen(tconn, 0, func(candidate string) uiauto.Action {
					return util.WaitForFieldTextToBe(tconn, inputField.Finder(), candidate)
				}),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.Name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.Name))

			if err := subtest.Action(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
