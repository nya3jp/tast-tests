// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import "chromiumos/tast/local/chrome/ime"

type handwritingMessage map[ime.Language]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message handwritingMessage) GetInputData(inputMethodCode ime.InputMethodCode) (InputData, bool) {
	var inputData InputData

	languageCode, ok := ime.LanguageOfIME[inputMethodCode]
	if !ok {
		return inputData, false
	}

	inputData, ok = message[languageCode]
	return inputData, ok
}

// HandwritingMessageHello defines hello handwriting messages of input methods
var HandwritingMessageHello = handwritingMessage{
	ime.EN: {
		HandwritingFile: "handwriting_en_hello.svg",
		ExpectedText:    "hello",
	},
	ime.ZH_HANS: {
		HandwritingFile: "handwriting_zh_hans_hello.svg",
		ExpectedText:    "你好",
	},
	ime.JA: {
		HandwritingFile: "handwriting_ja_hello.svg",
		ExpectedText:    "こんにちは",
	},
}
