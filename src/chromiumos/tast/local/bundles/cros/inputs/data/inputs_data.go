// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package data contains input data and expected outcome for input tests.
package data

import (
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
)

// InputData represents test data for input methods.
type InputData struct {
	// Character-based key sequences to tap on virtual keyboards.
	CharacterKeySeq []string
	// Location-based key sequences to tap on physical keyboards.
	LocationKeySeq []input.EventCode
	// Expected outcome text after input.
	ExpectedText string
	// Filename of .svg file containing handwriting strokes.
	HandwritingFile string
	// Filename of audio file containing the word voice.
	VoiceFile string
	// Whether select candidate from suggestion bar. Some IMEs need to manually
	// select from candidates to submit.
	SubmitFromSuggestion bool
}

// Message is a generic type that provides a function of retrieving input data
// by input methods.
type Message interface {
	GetInputData(im ime.InputMethod) (InputData, bool)
}

// LanguageOfIME matches an input method to a language. This mapping is
// intentionally for language-based handwriting and voice. Use with care
// for other scenarios, as this mapping may not be suitable.
var LanguageOfIME = map[ime.InputMethod]ime.Language{
	ime.EnglishUS:              ime.LANGUAGE_EN,      //NOLINT
	ime.ChinesePinyin:          ime.LANGUAGE_ZH_HANS, //NOLINT
	ime.Japanese:               ime.LANGUAGE_JA,      //NOLINT
	ime.JapaneseWithUSKeyboard: ime.LANGUAGE_JA,      //NOLINT
}

// ExtractExternalFiles returns the file names contained in messages for
// selected input methods.
func ExtractExternalFiles(messages []Message, inputMethods []ime.InputMethod) []string {
	var files = []string{}

	for _, message := range messages {
		for _, im := range inputMethods {
			//TODO(b/192819861): evaluate the OK result.
			inputData, _ := message.GetInputData(im)
			if inputData.HandwritingFile != "" {
				files = append(files, inputData.HandwritingFile)
			}
			if inputData.VoiceFile != "" {
				files = append(files, inputData.VoiceFile)
			}
		}
	}
	return files
}
