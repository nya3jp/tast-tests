// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// KeyboardEventWriter supports injecting events into a keyboard device.
type KeyboardEventWriter struct {
	rw   *RawEventWriter
	fast bool // if true, do not sleep after type; useful for unit tests
}

// Keyboard returns an EventWriter to inject events into an arbitrary keyboard device.
func Keyboard(ctx context.Context) (*KeyboardEventWriter, error) {
	infos, err := readDevices(procDevices)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %v", procDevices)
	}
	for _, info := range infos {
		if !info.isKeyboard() {
			continue
		}
		testing.ContextLogf(ctx, "Opening keyboard device %+v", info)
		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &KeyboardEventWriter{rw: device, fast: false}, nil
	}
	return nil, errors.New("didn't find keyboard device")
}

// sendKey writes a EV_KEY event containing the specified code and value, followed by a EV_SYN event.
// If firstErr points at a non-nil error, no events are written.
// If an error is encountered, it is saved to the address pointed to by firstErr.
func (kw *KeyboardEventWriter) sendKey(ec EventCode, val int32, firstErr *error) {
	if *firstErr == nil {
		*firstErr = kw.rw.Event(EV_KEY, ec, val)
	}
	if *firstErr == nil {
		*firstErr = kw.rw.Sync()
	}
}

// Type injects key events suitable for generating the string s.
// Only characters that can be typed using a QWERTY keyboard are supported,
// and the current keyboard layout must be QWERTY. The left Shift key is automatically
// pressed and released for uppercase letters or other characters that can be typed
// using Shift.
func (kw *KeyboardEventWriter) Type(ctx context.Context, s string) error {
	// Look up runes first so we can report an error before we start injecting events.
	type key struct {
		code    EventCode
		shifted bool
	}
	var keys []key
	for i, r := range []rune(s) {
		if code, ok := runeKeyCodes[r]; ok {
			keys = append(keys, key{code, false})
		} else if code, ok := shiftedRuneKeyCodes[r]; ok {
			keys = append(keys, key{code, true})
		} else {
			return errors.Errorf("unsupported rune %v at position %d", r, i)
		}
	}

	var firstErr error

	shifted := false
	for i, k := range keys {
		if k.shifted && !shifted {
			kw.sendKey(KEY_LEFTSHIFT, 1, &firstErr)
			shifted = true
		}

		kw.sendKey(k.code, 1, &firstErr)
		kw.sendKey(k.code, 0, &firstErr)

		if shifted && (i+1 == len(keys) || !keys[i+1].shifted) {
			kw.sendKey(KEY_LEFTSHIFT, 0, &firstErr)
			shifted = false
		}

		kw.sleepAfterType(ctx, &firstErr)
	}

	return firstErr
}

// Accel injects a sequence of key events simulating the accelerator (a.k.a. hotkey) described by s being typed.
// Accelerators are described as a sequence of '+'-separated, case-insensitive key characters or names.
// In addition to non-whitespace characters that are present on a QWERTY keyboard, the following key names may be used:
//	Modifiers:     "Ctrl", "Alt", "Search", "Shift"
//	Whitespace:    "Enter", "Space", "Tab", "Backspace"
//	Function keys: "F1", "F2", ..., "F12"
// "Shift" must be included for keys that are typed using Shift; for example, use "Ctrl+Shift+/" rather than "Ctrl+?".
func (kw *KeyboardEventWriter) Accel(ctx context.Context, s string) error {
	keys, err := parseAccel(s)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", s)
	}
	if len(keys) == 0 {
		return errors.Errorf("no keys found in %q", s)
	}

	// Press the keys in forward order and then release them in reverse order.
	var firstErr error
	for i := 0; i < len(keys); i++ {
		kw.sendKey(keys[i], 1, &firstErr)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		kw.sendKey(keys[i], 0, &firstErr)
	}
	kw.sleepAfterType(ctx, &firstErr)
	return firstErr
}

// sleepAfterType sleeps for short time. It is supposed to be called after key strokes.
// TODO(derat): Without sleeping between keystrokes, the omnibox seems to produce scrambled text.
// Figure out why. Presumably there's a bug in Chrome's input stack or the omnibox code.
func (kw *KeyboardEventWriter) sleepAfterType(ctx context.Context, firstErr *error) {
	if kw.fast {
		return
	}
	if *firstErr != nil {
		return
	}

	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		*firstErr = errors.Wrap(ctx.Err(), "timeout while typing")
	}
}

// Close closes the keyboard device.
func (kw *KeyboardEventWriter) Close() error {
	return kw.rw.Close()
}
