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

// VKInputMap contains sample input data of each input method.
// This is temporary solution to scale input method coverage.
// It might be refactored in b/188488890.
var VKInputMap = map[ime.InputMethodCode]vkInputData{
	ime.INPUTMETHOD_XKB_US_ENG: {
		TapKeySeq:    strings.Split("hello", ""),
		ExpectedText: "hello",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		TapKeySeq:    strings.Split("konnnitiha", ""),
		ExpectedText: "こんにちは",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		TapKeySeq:            strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
	ime.INPUTMETHOD_CANTONESE_CHINESE_TRADITIONAL: {
		TapKeySeq:            strings.Split("mou", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "冇",
	},
	ime.INPUTMETHOD_CANGJIE87_CHINESE_TRADITIONAL: {
		TapKeySeq:            strings.Split("竹手戈", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "我",
	},
}
