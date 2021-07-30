// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"strings"

	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
)

type typingMessage map[ime.InputMethod]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message typingMessage) GetInputData(im ime.InputMethod) (InputData, bool) {
	inputData, ok := message[im]
	return inputData, ok
}

// TypingMessageHello defines hello messages of input methods.
// TODO(b/192403778): Add location-based keySeq data for inputs test.
// TODO(b/192521170): Add data to cover that non US Eng tests would
// actually fail if run on the US keyboard
var TypingMessageHello = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: strings.Split("hello", ""),
		LocationKeySeq: []input.EventCode{input.KEY_H,
			input.KEY_E, input.KEY_L, input.KEY_L, input.KEY_O},
		ExpectedText: "hello",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		LocationKeySeq: []input.EventCode{input.KEY_K,
			input.KEY_O, input.KEY_N, input.KEY_N, input.KEY_N, input.KEY_I,
			input.KEY_T, input.KEY_I, input.KEY_H, input.KEY_A},
		ExpectedText: "こんにちは",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq: strings.Split("nihao", ""),
		LocationKeySeq: []input.EventCode{input.KEY_N,
			input.KEY_I, input.KEY_H, input.KEY_A, input.KEY_O},
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
	ime.EnglishUSWithInternationalKeyboard: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.EnglishUK: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.SpanishSpain: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.Swedish: {
		CharacterKeySeq: strings.Split("kött", ""),
		ExpectedText:    "kött",
	},
	ime.EnglishCanada: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.AlphanumericWithJapaneseKeyboard: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.Japanese: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.FrenchFrance: {
		CharacterKeySeq: strings.Split("bonjour", ""),
		ExpectedText:    "bonjour",
	},
	ime.Cantonese: {
		CharacterKeySeq:      strings.Split("mou", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "冇",
	},
	ime.ChineseCangjie: {
		CharacterKeySeq:      strings.Split("竹手戈", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "我",
	},
	ime.Korean: {
		CharacterKeySeq: []string{"ㅎ", "ᅡ", "ㄴ"},
		ExpectedText:    "한",
	},
	ime.Arabic: {
		CharacterKeySeq: strings.Split("سلام", ""),
		ExpectedText:    "سلام",
	},
}

// TypingMessagePassword defines messages of input methods for passwordInputField.
var TypingMessagePassword = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: strings.Split("hello", ""),
		ExpectedText:    "hello",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "konnnitiha",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq: strings.Split("nihao", ""),
		ExpectedText:    "nihao",
	},
}

// TypingMessageNumber defines messages of input methods for numberInputField.
var TypingMessageNumber = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq: strings.Split("-123.456", ""),
		ExpectedText:    "-123.456",
	},
}

// TypingMessageEmail defines messages of input methods for emailInputField.
// Add cover for special buttons on layouts of inputs methods that are
// currently not available when b/192515491 is resolved.
var TypingMessageEmail = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: []string{"t", "e", "s", "t", "@", "g", "m", "a", "i", "l", ".com"},
		ExpectedText:    "test@gmail.com",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq:      strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
}

// TypingMessageURL defines messages of input methods for urlInputField.
// Add cover for special buttons on layouts of inputs methods that are
// currently not available when b/192515491 is resolved.
var TypingMessageURL = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: []string{"g", "o", "o", "g", "l", "e", ".com", "/"},
		ExpectedText:    "google.com/",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: strings.Split("konnnitiha", ""),
		ExpectedText:    "こんにちは",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq:      strings.Split("nihao", ""),
		SubmitFromSuggestion: true,
		ExpectedText:         "你好",
	},
}

// TypingMessageTel defines messages of input methods for telInputField.
var TypingMessageTel = typingMessage{
	ime.EnglishUS: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "2", "3"},
		ExpectedText:    "-+,.(),;N123",
	},
	ime.JapaneseWithUSKeyboard: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
		ExpectedText:    "-+,.(),;N10",
	},
	ime.ChinesePinyin: {
		CharacterKeySeq: []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
		ExpectedText:    "-+,.(),;N10",
	},
}
