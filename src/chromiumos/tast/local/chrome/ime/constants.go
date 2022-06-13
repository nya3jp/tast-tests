// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

// ChromeIMEPrefix is the prefix of IME chrome extension.
const ChromeIMEPrefix = "_comp_ime_jkghodnilhceideoidjikpgommlajknk"

// ChromiumIMEPrefix is the prefix of IME chromium extension.
const ChromiumIMEPrefix = "_comp_ime_fgoepimhcoialccpbmpnnblemnepkkao"

// Language represents the handwriting/voice language for an input method.
type Language string

// List of languages, names are defined based on ISO 639.
const (
	LanguageAr      Language = "Arabic"
	LanguageCa      Language = "Catalan"
	LanguageEl      Language = "Greek"
	LanguageEn      Language = "English"
	LanguageEs      Language = "Spanish"
	LanguageFa      Language = "Persian"
	LanguageFr      Language = "French"
	LanguageGu      Language = "Gujarati"
	LanguageHi      Language = "Hindi"
	LanguageJa      Language = "Japanese"
	LanguageKn      Language = "Kannada"
	LanguageKo      Language = "Korean"
	LanguageMl      Language = "Malayalam"
	LanguageMr      Language = "Marathi"
	LanguageNe      Language = "Nepali"
	LanguageOr      Language = "Odia"
	LanguagePa      Language = "Punjabi"
	LanguageRu      Language = "Russian"
	LanguageSa      Language = "Sanskrit"
	LanguageSv      Language = "Swedish"
	LanguageTa      Language = "Tamil"
	LanguageTe      Language = "Telugu"
	LanguageUr      Language = "Urdu"
	LanguageYueHant Language = "Traditional Cantonese"
	LanguageZhHans  Language = "Simplified Chinese"
	LanguageZhHant  Language = "Traditional Chinese"
)
