// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

// InputMethodCode represents an input method code.
type InputMethodCode string

// List of input tool codes.
const (
	INPUTMETHOD_NACL_MOZC_US              InputMethodCode = "nacl_mozc_us"    // Japanese input (for US keyboard)
	INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED InputMethodCode = "zh-t-i0-pinyin"  // Chinese Pinyin input method
	INPUTMETHOD_XKB_US_ENG                InputMethodCode = "xkb:us::eng"     // English US keyboard
	INPUTMETHOD_XKB_US_INTL               InputMethodCode = "xkb:us:intl:eng" // US International keyboard
	INPUTMETHOD_XKB_FR_FRA                InputMethodCode = "xkb:fr::fra"     // FR France keyboard
)
