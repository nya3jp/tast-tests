// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import "chromiumos/tast/local/chrome/ime"

type voiceMessage map[ime.Language]InputData

// GetInputData returns two values given an input method: inputData and ok.
// If the test data for the given input method can be found, ok is true.
// If there is no match for the given input method, ok is false, and inputData is the zero value.
func (message voiceMessage) GetInputData(im ime.InputMethod) (InputData, bool) {
	inputData, ok := message[im.VoiceLanguage]
	return inputData, ok
}

// VoiceMessageHello defines hello voice messages of input methods.
var VoiceMessageHello = voiceMessage{
	ime.LanguageAr: {
		VoiceFile:    "voice_ar_hello.wav",
		ExpectedText: "السلام عليكم",
	},
	ime.LanguageEn: {
		VoiceFile:    "voice_en_hello.wav",
		ExpectedText: "hello",
	},
	ime.LanguageJa: {
		VoiceFile:    "voice_ja_hello.wav",
		ExpectedText: "こんにちは",
	},
	ime.LanguageKo: {
		VoiceFile:    "voice_ko_hello.wav",
		ExpectedText: "안녕하세요",
	},
	ime.LanguageZhHans: {
		VoiceFile:    "voice_zh_hans_hello.wav",
		ExpectedText: "你好",
	},
}
