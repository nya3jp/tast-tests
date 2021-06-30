// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

// InputMethodCode represents an input method code.
type InputMethodCode string

// List of input tool codes.
// Sorted by IME usage stat go/e14s-grades.
const (
	INPUTMETHOD_XKB_US_ENG                    InputMethodCode = "xkb:us::eng"               // NOLINT: English (US)
	INPUTMETHOD_XKB_US_INTL                   InputMethodCode = "xkb:us:intl:eng"           // NOLINT: English (US) with International keyboard
	INPUTMETHOD_XKB_GB_EXTD_ENG               InputMethodCode = "xkb:gb:extd:eng"           // NOLINT: English (UK)
	INPUTMETHOD_XKB_ES_SPA                    InputMethodCode = "xkb:es::spa"               // NOLINT: Spanish (Spain)
	INPUTMETHOD_XKB_SE_SWE                    InputMethodCode = "xkb:se::swe"               // NOLINT: Swedish
	INPUTMETHOD_XKB_JP_JPN                    InputMethodCode = "xkb:jp::jpn"               // NOLINT: Alphanumeric with Japanese keyboard
	INPUTMETHOD_XKB_CA_ENG                    InputMethodCode = "xkb:ca:eng:eng"            // NOLINT: English (Canada)
	INPUTMETHOD_NACL_MOZC_JP                  InputMethodCode = "nacl_mozc_jp"              // NOLINT: Japanese
	INPUTMETHOD_XKB_FR_FRA                    InputMethodCode = "xkb:fr::fra"               // NOLINT: Franch (France)
	INPUTMETHOD_NACL_MOZC_US                  InputMethodCode = "nacl_mozc_us"              // NOLINT: Japanese with US keyboard
	INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED     InputMethodCode = "zh-t-i0-pinyin"            // NOLINT: Chinese Pinyin input method
	INPUTMETHOD_CANTONESE_CHINESE_TRADITIONAL InputMethodCode = "yue-hant-t-i0-und"         // NOLINT: Chinese Cantonese input method
	INPUTMETHOD_CANGJIE87_CHINESE_TRADITIONAL InputMethodCode = "zh-hant-t-i0-cangjie-1987" // NOLINT: Chinese Cangjie input method
	INPUTMETHOD_HANGEUL_HANJA_KOREAN          InputMethodCode = "ko-t-i0-und"               // NOLINT: Korean input method
)

// Language represents a language.
type Language string

// List of languages.
const (
	EN      Language = "English"
	ZH_HANS Language = "Simplified Chinese"
	JA      Language = "Japanese"
)

// LanguageOfIME matches an input method to a language. This mapping is
// intentionaly for language-based handwriting and voice. Use with care
// for other scenarios, as this mapping may not be suitable.
var LanguageOfIME = map[InputMethodCode]Language{
	INPUTMETHOD_XKB_US_ENG:                EN,
	INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED: ZH_HANS,
	INPUTMETHOD_NACL_MOZC_JP:              JA,
	INPUTMETHOD_NACL_MOZC_US:              JA,
}
