// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

import "chromiumos/tast/errors"

// TODO(b/192819861): Defining new input method struct and migrating existing use of InputMethodCode.
// This page is partly implementation of b/192819861.
// This structure might need to be refined.

// InputMethod is a strcuture representing input method.
type InputMethod struct {
	Name    string // The displayed name of the IME in OS Settings.
	IMECode string // The program code of the IME, e.g. "xkb:us::eng"
}

// XKB_US_ENG represents the input method of English (US).
var XKB_US_ENG = InputMethod{ // NOLINT
	Name:    "English (US)",
	IMECode: "xkb:us::eng",
}

// XKB_US_INTL represents the input method of English (US) with International keyboard.
var XKB_US_INTL = InputMethod{ // NOLINT
	Name:    "English (US) with International keyboard",
	IMECode: "xkb:us:intl:eng",
}

// XKB_GB_EXTD_ENG represents the input method of English (US).
var XKB_GB_EXTD_ENG = InputMethod{ // NOLINT
	Name:    "English (UK)",
	IMECode: "xkb:gb:extd:eng",
}

// XKB_ES_SPA represents the input method of Spanish (Spain).
var XKB_ES_SPA = InputMethod{ // NOLINT
	Name:    "Spanish (Spain)",
	IMECode: "xkb:es::spa",
}

// XKB_SE_SWE represents the input method of Swedish.
var XKB_SE_SWE = InputMethod{ // NOLINT
	Name:    "Swedish",
	IMECode: "xkb:se::swe",
}

// XKB_JP_JPN represents the input method of Alphanumeric with Japanese keyboard.
var XKB_JP_JPN = InputMethod{ // NOLINT
	Name:    "Alphanumeric with Japanese keyboard",
	IMECode: "xkb:jp::jpn",
}

// XKB_CA_ENG represents the input method of English (Canada).
var XKB_CA_ENG = InputMethod{ // NOLINT
	Name:    "English (Canada)",
	IMECode: "xkb:ca:eng:eng",
}

// NACL_MOZC_JP represents the input method of Japanese.
var NACL_MOZC_JP = InputMethod{ // NOLINT
	Name:    "Japanese",
	IMECode: "nacl_mozc_jp",
}

// XKB_FR_FRA represents the input method of Franch (France).
var XKB_FR_FRA = InputMethod{ // NOLINT
	Name:    "Franch (France)",
	IMECode: "xkb:fr::fra",
}

// NACL_MOZC_US represents the input method of Japanese with US keyboard.
var NACL_MOZC_US = InputMethod{ // NOLINT
	Name:    "Japanese with US keyboard",
	IMECode: "nacl_mozc_us",
}

// PINYIN_CHINESE_SIMPLIFIED represents the input method of Chinese Pinyin.
var PINYIN_CHINESE_SIMPLIFIED = InputMethod{ // NOLINT
	Name:    "Chinese Pinyin",
	IMECode: "zh-t-i0-pinyin",
}

// CANTONESE_CHINESE_TRADITIONAL represents the input method of Chinese Cantonese.
var CANTONESE_CHINESE_TRADITIONAL = InputMethod{ // NOLINT
	Name:    "Chinese Cantonese",
	IMECode: "yue-hant-t-i0-und",
}

// CANGJIE87_CHINESE_TRADITIONAL represents the input method of Chinese Cangjie.
var CANGJIE87_CHINESE_TRADITIONAL = InputMethod{ // NOLINT
	Name:    "Chinese Cangjie",
	IMECode: "zh-hant-t-i0-cangjie-1987",
}

// HANGEUL_HANJA_KOREAN represents the input method of Korean.
var HANGEUL_HANJA_KOREAN = InputMethod{ // NOLINT
	Name:    "Korean",
	IMECode: "ko-t-i0-und",
}

// inputMethods represents in-use (available) IMEs in ChromeOS.
// Only listed input methods are promised to be available.
var inputMethods = []InputMethod{ // NOLINT
	XKB_US_ENG,
	XKB_US_INTL,
	XKB_GB_EXTD_ENG,
	XKB_ES_SPA,
	XKB_SE_SWE,
	XKB_JP_JPN,
	XKB_CA_ENG,
	NACL_MOZC_JP,
	XKB_FR_FRA,
	NACL_MOZC_US,
	PINYIN_CHINESE_SIMPLIFIED,
	CANTONESE_CHINESE_TRADITIONAL,
	CANGJIE87_CHINESE_TRADITIONAL,
	HANGEUL_HANJA_KOREAN,
}

// FindInputMethodByName finds the input method by displayed name.
func FindInputMethodByName(name string) (*InputMethod, error) {
	for _, im := range inputMethods {
		if im.Name == name {
			return &im, nil
		}
	}
	return nil, errors.Errorf("failed to find input method by name %q", name)
}

// FindInputMethodByCode finds the input method by ime code.
func FindInputMethodByCode(imeCode string) (*InputMethod, error) {
	for _, im := range inputMethods {
		if im.IMECode == imeCode {
			return &im, nil
		}
	}
	return nil, errors.Errorf("failed to find input method by IME code %q", imeCode)
}
