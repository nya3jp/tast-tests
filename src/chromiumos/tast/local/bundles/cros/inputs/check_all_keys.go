// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckAllKeys,
		Desc: "Checks virtual keyboard functionalities of language EnglishUS, EnglishUK, Alphanumeric With JapaneseKeyboard, Japanese With US Keyboard and Japanese",
		Contacts: []string{
			"lance.wang@cienet.com", // Author
			"shengjun@google.com",   // PoC
			"cienet-development@googlegroups.com",
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.VKEnabledTabletReset,
		Timeout:      20 * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			}},
	})
}

// CheckAllKeys tests virtual keyboard features for different languages.
func CheckAllKeys(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		its.ClosePage(ctx)
		its.Close()
	}(cleanupCtx)

	for _, im := range []ime.InputMethod{
		ime.Japanese,
		ime.JapaneseWithUSKeyboard,
		ime.AlphanumericWithJapaneseKeyboard,
		ime.EnglishUS,
		ime.EnglishUK,
	} {
		f := func(ctx context.Context, s *testing.State) {
			testing.ContextLog(ctx, "Switching input method to ", im.Name)
			if err := im.InstallAndActivate(tconn)(ctx); err != nil {
				s.Fatalf("Failed to switch to input method [%s]: %v", im.Name, err)
			}

			uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

			for _, detail := range inputTests(im) {
				if len(detail.characters) == 0 {
					continue
				}

				if err := its.ClickFieldUntilVKShown(testserver.TextAreaInputField)(ctx); err != nil {
					s.Fatal("Failed to click text area until virtual keyboard is shown: ", err)
				}

				if err := switchInputOption(vkbCtx, detail.chType)(ctx); err != nil {
					s.Fatal("Failed to switch input option: ", err)
				}

				s.Log("Verifying vk basic input functionality, ", detail.chType)
				for key, expected := range detail.characters {
					if err := uiauto.Combine(fmt.Sprintf("validate virtual keyboard key %q", key),
						its.ClickField(testserver.TextAreaInputField),
						its.Clear(testserver.TextAreaInputField),
						vkbCtx.TapKeyIgnoringCase(key),
						util.WaitForFieldTextToBeIgnoringCase(tconn, testserver.TextAreaInputField.Finder(), expected),
						vkbCtx.TapKeyIgnoringCase("enter"),
					)(ctx); err != nil {
						s.Fatal("Failed to complete tests: ", err)
					}
				}

				// Leave virtual keyboard after each test set is done.
				if err := vkbCtx.TapKeyIgnoringCase("hide keyboard")(ctx); err != nil {
					s.Fatal("Failed to hide virtual keyboard: ", err)
				}
			}
		}

		if !s.Run(ctx, fmt.Sprintf("verify virtual keyboard subcase %q", im.Name), f) {
			s.Errorf("Failed to complete test of verifying virtual keyboard %q", im.Name)
		}
	}
}

type characterType string

const (
	letter      characterType = "normal letters"
	number      characterType = "numbers"
	symbol      characterType = "basic symbols"
	symbolExtra characterType = "extra symbols"
)

func switchInputOption(vkbCtx *vkb.VirtualKeyboardContext, chType characterType) uiauto.Action {
	switch chType {
	case number, symbol:
		return vkbCtx.TapKey("switch to symbols")
	case symbolExtra:
		return uiauto.Combine("switch to more symbols",
			vkbCtx.TapKey("switch to symbols"),
			vkbCtx.TapNode(vkb.KeyFinder.Name("switch to more symbols").First()), // Multiple nodes exist, cannot use vkbCtx.TapKey.
		)
	}
	return func(context.Context) error { return nil }
}

type testDetail struct {
	chType     characterType
	characters map[string]string // characters is the map of vk-input and expected result.
}

func inputTests(inputMethod ime.InputMethod) []*testDetail {
	return []*testDetail{
		letterTest(inputMethod),
		numberTest(inputMethod),
		symbolsBasicTest(inputMethod),
		symbolsExtraTest(inputMethod),
	}
}

func letterTest(inputMethod ime.InputMethod) *testDetail {
	const allLetters = "qwertyuiopasdfghjklzxcvbnm"
	vkToType := strings.Split(allLetters, "")

	td := &testDetail{
		chType:     letter,
		characters: make(map[string]string),
	}
	for _, s := range vkToType {
		td.characters[s] = s
	}

	switch inputMethod {
	case ime.Japanese, ime.JapaneseWithUSKeyboard:
		td.characters = map[string]string{
			"q": "ｑ",
			"w": "ｗ",
			"e": "え",
			"r": "ｒ",
			"t": "ｔ",
			"y": "ｙ",
			"u": "う",
			"i": "い",
			"o": "お",
			"p": "ｐ",
			"a": "あ",
			"s": "ｓ",
			"d": "ｄ",
			"f": "ｆ",
			"g": "ｇ",
			"h": "ｈ",
			"j": "ｊ",
			"k": "ｋ",
			"l": "ｌ",
			"z": "ｚ",
			"x": "ｘ",
			"c": "ｃ",
			"v": "ｖ",
			"b": "ｂ",
			"n": "ｎ",
			"m": "ｍ",
		}
	}

	return td
}

func numberTest(inputMethod ime.InputMethod) *testDetail {
	const allNumbers = "1234567890"
	vkToType := strings.Split(allNumbers, "")

	td := &testDetail{
		chType:     number,
		characters: make(map[string]string),
	}
	for _, s := range vkToType {
		td.characters[s] = s
	}

	switch inputMethod {
	case ime.JapaneseWithUSKeyboard, ime.Japanese:
		td.characters = map[string]string{
			"1": "１",
			"2": "２",
			"3": "３",
			"4": "４",
			"5": "５",
			"6": "６",
			"7": "７",
			"8": "８",
			"9": "９",
			"0": "０",
		}
	}

	return td
}

func symbolsBasicTest(inputMethod ime.InputMethod) *testDetail {
	const basicSymbols = "@#$%&-+()\\=*\"':;!?_/,."
	vkToType := strings.Split(basicSymbols, "")

	td := &testDetail{
		chType:     symbol,
		characters: make(map[string]string),
	}
	for _, s := range vkToType {
		td.characters[s] = s
	}

	jpkbExpectedChars := map[string]string{
		"@":  "＠",
		"#":  "＃",
		"$":  "＄",
		"%":  "％",
		"&":  "＆",
		"-":  "ー",
		"+":  "＋",
		"(":  "（",
		")":  "）",
		"\\": "￥",
		"=":  "＝",
		"*":  "＊",
		"\"": "”",
		"'":  "’",
		":":  "：",
		";":  "；",
		"!":  "！",
		"?":  "？",
		"_":  "＿",
		"/":  "・",
		"、":  "、",
		"。":  "。",
	}

	switch inputMethod {
	case ime.EnglishUK:
		delete(td.characters, "$")
		td.characters["£"] = "£"
	case ime.JapaneseWithUSKeyboard:
		td.characters = jpkbExpectedChars
	case ime.Japanese:
		td.characters = jpkbExpectedChars
		td.characters["_"] = "ー"
	}

	return td
}

func symbolsExtraTest(inputMethod ime.InputMethod) *testDetail {
	const extraSymbols = "~`|•√π÷×¶Δ£¢€¥^°={}\\©®™℅[]¡¿<>,."
	vkToType := strings.Split(extraSymbols, "")

	jpkbExpectedChars := make(map[string]string)
	td := &testDetail{
		chType:     symbolExtra,
		characters: make(map[string]string),
	}
	for _, s := range vkToType {
		td.characters[s] = s
		jpkbExpectedChars[s] = s
	}

	jpkbExpectedChars["~"] = "〜"
	jpkbExpectedChars["`"] = "｀"
	jpkbExpectedChars["|"] = "｜"
	jpkbExpectedChars["^"] = "＾"
	jpkbExpectedChars["="] = "＝"
	jpkbExpectedChars["{"] = "｛"
	jpkbExpectedChars["}"] = "｝"
	jpkbExpectedChars["\\"] = "￥"
	jpkbExpectedChars["["] = "「"
	jpkbExpectedChars["]"] = "」"
	jpkbExpectedChars["<"] = "＜"
	jpkbExpectedChars[">"] = "＞"
	delete(jpkbExpectedChars, ",")
	delete(jpkbExpectedChars, ".")
	jpkbExpectedChars["、"] = "、"
	jpkbExpectedChars["。"] = "。"

	switch inputMethod {
	case ime.EnglishUK:
		delete(td.characters, "£")
		td.characters["$"] = "$"
	case ime.JapaneseWithUSKeyboard:
		td.characters = jpkbExpectedChars
	case ime.Japanese:
		jpkbExpectedChars["¥"] = "￥"
		td.characters = jpkbExpectedChars
	}

	return td
}
