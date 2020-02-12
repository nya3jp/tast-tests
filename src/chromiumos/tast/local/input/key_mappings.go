// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"strings"

	"chromiumos/tast/errors"
)

// runeKeyCodes contains runes that can be typed with a single key
// (in the default QWERTY layout).
var runeKeyCodes = map[rune]EventCode{
	'1':    KEY_1,
	'2':    KEY_2,
	'3':    KEY_3,
	'4':    KEY_4,
	'5':    KEY_5,
	'6':    KEY_6,
	'7':    KEY_7,
	'8':    KEY_8,
	'9':    KEY_9,
	'0':    KEY_0,
	'-':    KEY_MINUS,
	'=':    KEY_EQUAL,
	'\b':   KEY_BACKSPACE,
	'\t':   KEY_TAB,
	'q':    KEY_Q,
	'w':    KEY_W,
	'e':    KEY_E,
	'r':    KEY_R,
	't':    KEY_T,
	'y':    KEY_Y,
	'u':    KEY_U,
	'i':    KEY_I,
	'o':    KEY_O,
	'p':    KEY_P,
	'[':    KEY_LEFTBRACE,
	']':    KEY_RIGHTBRACE,
	'\n':   KEY_ENTER,
	'a':    KEY_A,
	's':    KEY_S,
	'd':    KEY_D,
	'f':    KEY_F,
	'g':    KEY_G,
	'h':    KEY_H,
	'j':    KEY_J,
	'k':    KEY_K,
	'l':    KEY_L,
	';':    KEY_SEMICOLON,
	'\'':   KEY_APOSTROPHE,
	'`':    KEY_GRAVE,
	'\\':   KEY_BACKSLASH,
	'z':    KEY_Z,
	'x':    KEY_X,
	'c':    KEY_C,
	'v':    KEY_V,
	'b':    KEY_B,
	'n':    KEY_N,
	'm':    KEY_M,
	',':    KEY_COMMA,
	'.':    KEY_DOT,
	'/':    KEY_SLASH,
	' ':    KEY_SPACE,
	'\x1b': KEY_ESC,
}

// runeKeyCodes contains runes that can be typed by holding Shift and pressing a
// single key (in the default QWERTY layout).
var shiftedRuneKeyCodes = map[rune]EventCode{
	'!': KEY_1,
	'@': KEY_2,
	'#': KEY_3,
	'$': KEY_4,
	'%': KEY_5,
	'^': KEY_6,
	'&': KEY_7,
	'*': KEY_8,
	'(': KEY_9,
	')': KEY_0,
	'_': KEY_MINUS,
	'+': KEY_EQUAL,
	'Q': KEY_Q,
	'W': KEY_W,
	'E': KEY_E,
	'R': KEY_R,
	'T': KEY_T,
	'Y': KEY_Y,
	'U': KEY_U,
	'I': KEY_I,
	'O': KEY_O,
	'P': KEY_P,
	'{': KEY_LEFTBRACE,
	'}': KEY_RIGHTBRACE,
	'A': KEY_A,
	'S': KEY_S,
	'D': KEY_D,
	'F': KEY_F,
	'G': KEY_G,
	'H': KEY_H,
	'J': KEY_J,
	'K': KEY_K,
	'L': KEY_L,
	':': KEY_SEMICOLON,
	'"': KEY_APOSTROPHE,
	'~': KEY_GRAVE,
	'|': KEY_BACKSLASH,
	'Z': KEY_Z,
	'X': KEY_X,
	'C': KEY_C,
	'V': KEY_V,
	'B': KEY_B,
	'N': KEY_N,
	'M': KEY_M,
	'<': KEY_COMMA,
	'>': KEY_DOT,
	'?': KEY_SLASH,
}

// namedKeyCodes contains multi-character names describing keys that may be used in accelerators.
var namedKeyCodes = map[string]EventCode{
	"alt":    KEY_LEFTALT,
	"ctrl":   KEY_LEFTCTRL,
	"search": KEY_LEFTMETA,
	"shift":  KEY_LEFTSHIFT,

	"backspace": KEY_BACKSPACE,
	"end":       KEY_END,
	"enter":     KEY_ENTER,
	"home":      KEY_HOME,
	"space":     KEY_SPACE,
	"tab":       KEY_TAB,

	"f1":  KEY_F1,
	"f2":  KEY_F2,
	"f3":  KEY_F3,
	"f4":  KEY_F4,
	"f5":  KEY_F5,
	"f6":  KEY_F6,
	"f7":  KEY_F7,
	"f8":  KEY_F8,
	"f9":  KEY_F9,
	"f10": KEY_F10,
	"f11": KEY_F11,
	"f12": KEY_F12,

	"playpause": KEY_PLAYPAUSE,
}

// parseAccel parses a string in the format accepted by the Accel function.
// It returns keycodes in the order in which they appear in the string.
func parseAccel(accel string) ([]EventCode, error) {
	var keys []EventCode
	for _, name := range strings.Split(accel, "+") {
		if len(name) == 0 {
			return nil, errors.New("empty key")
		}

		lname := strings.ToLower(name)
		if code, ok := namedKeyCodes[lname]; ok {
			keys = append(keys, code)
			continue
		}

		if runes := []rune(lname); len(runes) == 1 {
			if code, ok := runeKeyCodes[runes[0]]; ok {
				// Require whitespace chars to be spelled out.
				if code == KEY_BACKSPACE || code == KEY_TAB || code == KEY_ENTER || code == KEY_SPACE {
					return nil, errors.Errorf("must spell out key name instead of using %q", name)
				}
				keys = append(keys, code)
				continue
			}
		}

		return nil, errors.Errorf("unknown key %q", name)
	}
	return keys, nil
}
