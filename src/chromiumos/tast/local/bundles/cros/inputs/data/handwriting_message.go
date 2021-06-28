// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import "chromiumos/tast/local/chrome/ime"

type handwritingMessage map[ime.LanguageCode]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message handwritingMessage) GetInputData(inputMethodCode ime.InputMethodCode) (InputData, bool) {
	var inputData InputData

	languageCode, ok := ime.LanguageCodeOfIME[inputMethodCode]
	if !ok {
		return inputData, false
	}

	inputData, ok = message[languageCode]
	return inputData, ok
}

// HandwritingMessageHello defines hello handwriting messages of input methods
var HandwritingMessageHello = handwritingMessage{
	ime.EN: {
		HandwritingFile: "handwriting_en_hello_20210129.svg",
		ExpectedText:    "hello",
	},
	ime.CN_SIMPLIFIED: {
		HandwritingFile: "handwriting_cn_hello_20210129.svg",
		ExpectedText:    "你好",
	},
	ime.JP: {
		HandwritingFile: "handwriting_jp_hello_20210129.svg",
		ExpectedText:    "こんにちは",
	},
}
