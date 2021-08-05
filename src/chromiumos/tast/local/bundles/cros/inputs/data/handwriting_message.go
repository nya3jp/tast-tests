// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import "chromiumos/tast/local/chrome/ime"

type handwritingMessage map[string]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message handwritingMessage) GetInputData(im ime.InputMethod) (InputData, bool) {
	inputData, ok := message[im.Language]
	return inputData, ok
}

// HandwritingMessageHello defines hello handwriting messages of input methods
var HandwritingMessageHello = handwritingMessage{
	ime.LanguageAr: {
		HandwritingFile: "handwriting_ar_hello.svg",
		ExpectedText:    "سلام",
	},
	ime.LanguageEn: {
		HandwritingFile: "handwriting_en_hello.svg",
		ExpectedText:    "hello",
	},
	ime.LanguageJa: {
		HandwritingFile: "handwriting_ja_hello.svg",
		ExpectedText:    "こんにちは",
	},
	ime.LanguageKo: {
		HandwritingFile: "handwriting_ko_hello.svg",
		ExpectedText:    "안녕",
	},
	ime.LanguageZhHans: {
		HandwritingFile: "handwriting_zh_hans_hello.svg",
		ExpectedText:    "你好",
	},
}
