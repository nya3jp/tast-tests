// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"strings"

	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
)

type typingMessage map[ime.InputMethodCode]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message typingMessage) GetInputData(inputMethodCode ime.InputMethodCode) (InputData, bool) {
	inputData, ok := message[inputMethodCode]
	return inputData, ok
}

// TypingMessageHello defines hello messages of input methods.
// TODO(b/192403778): Add location-based keySeq data for inputs test.
var TypingMessageHello = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: strings.Split("hello", ""),
		LocationKeySeq: []input.EventCode{input.KEY_H,
			input.KEY_E, input.KEY_L, input.KEY_L, input.KEY_O},
		ExpectedText: "hello",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		LocationKeySeq: []input.EventCode{input.KEY_K,
			input.KEY_O, input.KEY_N, input.KEY_N, input.KEY_N, input.KEY_I,
			input.KEY_T, input.KEY_I, input.KEY_H, input.KEY_A},
		ExpectedText: "こんにちは",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq: strings.Split("nihao", ""),
		LocationKeySeq: []input.EventCode{input.KEY_N,
			input.KEY_I, input.KEY_H, input.KEY_A, input.KEY_O},
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
	ime.INPUTMETHOD_XKB_US_INTL: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_XKB_GB_EXTD_ENG: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_XKB_ES_SPA: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_XKB_SE_SWE: {
		CharacterKeySeq:      strings.Split("kött", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "kött",
	},
	ime.INPUTMETHOD_XKB_CA_ENG: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_XKB_JP_JPN: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_NACL_MOZC_JP: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.INPUTMETHOD_XKB_FR_FRA: {
		CharacterKeySeq: strings.Split("bonjour", ""),
		ExpectedText:    "bonjour",
	},
	ime.INPUTMETHOD_CANTONESE_CHINESE_TRADITIONAL: {
		CharacterKeySeq:      strings.Split("mou", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "冇",
	},
	ime.INPUTMETHOD_CANGJIE87_CHINESE_TRADITIONAL: {
		CharacterKeySeq:      strings.Split("竹手戈", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "我",
	},
	ime.INPUTMETHOD_HANGEUL_HANJA_KOREAN: {
		CharacterKeySeq: []string{"ㅎ", "ᅡ", "ㄴ"}, // ㅎᅡㄴ
		ExpectedText:    "한",
	},
}

// TypingMessagePassword defines messages of input methods for passwordInputField.
var TypingMessagePassword = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "konnnitiha",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq: strings.Split("nihao", ""),
		ExpectedText:    "nihao",
	},
}

// TypingMessageNumber defines messages of input methods for numberInputField.
var TypingMessageNumber = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
}

// TypingMessageEmail defines messages of input methods for emailInputField.
var TypingMessageEmail = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: []string{"t", "e", "s", "t", "@", "g", "m", "a", "i", "l", ".com"},
		ExpectedText:    "test@gmail.com",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq:      strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
}

// TypingMessageURL defines messages of input methods for urlInputField.
var TypingMessageURL = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: []string{"g", "o", "o", "g", "l", "e", ".com", "/"},
		ExpectedText:    "google.com/",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq:      strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
}

// TypingMessageTel defines messages of input methods for telInputField.
var TypingMessageTel = typingMessage{
	ime.INPUTMETHOD_XKB_US_ENG: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "2", "3"},
		ExpectedText:    "-+,.(),;N123",
	},
	ime.INPUTMETHOD_NACL_MOZC_US: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
		ExpectedText:    "-+,.(),;N10",
	},
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
		ExpectedText:    "-+,.(),;N10",
	},
}
