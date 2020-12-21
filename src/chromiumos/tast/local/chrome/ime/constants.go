// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

// InputMethodCode represents an input method code.
type InputMethodCode string

// List of input tool codes.
const (
	INPUTMETHOD_NACL_MOZC_US                  InputMethodCode = "nacl_mozc_us"              // Japanese input (for US keyboard)
	INPUTMETHOD_NACL_MOZC_JP                  InputMethodCode = "nacl_mozc_jp"              // Japanese input (for JP keyboard)
	INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED     InputMethodCode = "zh-t-i0-pinyin"            // Chinese Pinyin input method
	INPUTMETHOD_CANTONESE_CHINESE_TRADITIONAL InputMethodCode = "yue-hant-t-i0-und"         // Chinese Cantonese input method
	INPUTMETHOD_CANGJIE87_CHINESE_TRADITIONAL InputMethodCode = "zh-hant-t-i0-cangjie-1987" // Chinese Cangjie input method
	INPUTMETHOD_HANGUL_KOREAN                 InputMethodCode = "ko-t-i0-und"               // Korean input method
	INPUTMETHOD_XKB_US_ENG                    InputMethodCode = "xkb:us::eng"               // English US keyboard
	INPUTMETHOD_XKB_US_INTL                   InputMethodCode = "xkb:us:intl:eng"           // US International keyboard
	INPUTMETHOD_XKB_FR_FRA                    InputMethodCode = "xkb:fr::fra"               // FR France keyboard
	INPUTMETHOD_XKB_ES_SPA                    InputMethodCode = "xkb:es::spa"               // MSG Keyboard Spanish
	INPUTMETHOD_XKB_JP_JPN                    InputMethodCode = "xkb:jp::jpn"               // MSG Keyboard Japanese
)
