// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package data contains input data and expected outcome for input tests.
package data

import (
	"strings"

	"chromiumos/tast/local/chrome/ime"
)

type vkInputData struct {
	// Key sequences to tap on virtual keyboard.
	TapKeySeq []string
	// Whether select candidate from suggestion bar.
	SubmitFromSuggestion bool
	// Expected outcome text after input.
	ExpectedText string
	// Whether skip it from test execution.
	// It is a flag to temporarily exclude input method if it fails due to a non-testdata problem.
	SkipTest bool
}

var vkInputEng = vkInputData{
	TapKeySeq:    strings.Split("hello", ""),
	ExpectedText: "hello",
}

// VKInputMap contains sample input data of each input method.
// This is temporary solution to scale input method coverage.
// It might be refactored in b/188488890.
var VKInputMap = map[ime.InputMethod]vkInputData{
	ime.EnglishUS:                          vkInputEng,
	ime.EnglishUSWithInternationalKeyboard: vkInputEng,
	ime.EnglishUK:                          vkInputEng,
	ime.SpanishSpain:                       vkInputEng,
	ime.Swedish: {
		TapKeySeq:            strings.Split("kött", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "kött",
	},
	ime.EnglishCanada:                    vkInputEng,
	ime.AlphanumericWithJapaneseKeyboard: vkInputEng,
	ime.Japanese: {
		TapKeySeq:    strings.Split("konnnitiha", ""),
		ExpectedText: "こんにちは",
	},
	ime.JapaneseWithUSKeyboard: {
		TapKeySeq:    strings.Split("konnnitiha", ""),
		ExpectedText: "こんにちは",
	},
	ime.FrenchFrance: {
		TapKeySeq:    strings.Split("bonjour", ""),
		ExpectedText: "bonjour",
	},
	ime.ChinesePinyin: {
		TapKeySeq:            strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
	ime.Cantonese: {
		TapKeySeq:            strings.Split("mou", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "冇",
	},
	ime.ChineseCangjie: {
		TapKeySeq:            strings.Split("竹手戈", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "我",
	},
	ime.Korean: {
		TapKeySeq:    []string{"ㅎ", "\u1161", "ㄴ"}, // ㅎㅏㄴ
		ExpectedText: "한",
	},
}
