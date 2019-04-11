// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// KeyboardEventWriter supports injecting events into a keyboard device.
type KeyboardEventWriter struct {
	rw   *RawEventWriter
	virt *os.File // if non-nil, used to hold a virtual device open
	fast bool     // if true, do not sleep after type; useful for unit tests
	dev  string   // path to underlying device in /dev/input
}

var nextVirtKbdNum = 1 // appended to virtual keyboard device name

// Keyboard returns an EventWriter to inject events into an arbitrary keyboard device.
//
// If a physical keyboard is present, it is used.
// Otherwise, a one-off virtual device is created.
func Keyboard(ctx context.Context) (*KeyboardEventWriter, error) {
	// Look for an existing physical keyboard first, but only if we're not in tablet mode,
	// as the EC may mask keyboard events in that case: https://crbug.com/930568
	if sw, err := querySwitch(ctx, SW_TABLET_MODE); err != nil {
		return nil, errors.Wrap(err, "failed to get tablet mode state")
	} else if sw == switchOn {
		testing.ContextLog(ctx, "In tablet mode, so not looking for physical keyboard")
	} else {
		infos, err := readDevices("")
		if err != nil {
			return nil, errors.Wrap(err, "failed to read devices")
		}
		for _, info := range infos {
			if info.isKeyboard() && info.phys != "" {
				testing.ContextLogf(ctx, "Using existing keyboard device %+v", info)

				rw, err := Device(ctx, info.path)
				if err != nil {
					return nil, err
				}
				return &KeyboardEventWriter{rw: rw, dev: info.path}, nil
			}
		}
	}

	// If we didn't find a real keyboard, create a virtual one.
	return VirtualKeyboard(ctx)
}

// VirtualKeyboard creates a virtual keyboard device and returns an EventWriter that injects events into it.
func VirtualKeyboard(ctx context.Context) (*KeyboardEventWriter, error) {
	kw := &KeyboardEventWriter{}

	// Include our PID in the device name to be extra careful in case an old bundle process hasn't exited.
	name := fmt.Sprintf("Tast virtual keyboard %d.%d", os.Getpid(), nextVirtKbdNum)
	nextVirtKbdNum++
	testing.ContextLogf(ctx, "Creating virtual keyboard device %q", name)

	// These values are copied from the "AT Translated Set 2 keyboard" device on an amd64-generic VM.
	// The one exception is the bus, which we hardcode as USB, as 0x11 (BUS_I8042) doesn't work on some hardware.
	// See https://crrev.com/c/1407138 for more discussion.
	const usbBus = 0x3 // BUS_USB from input.h
	var err error
	if kw.dev, kw.virt, err = createVirtual(name, devID{usbBus, 0x1, 0x1, 0xab41}, 0, 0x120013,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x402000000, 0x3803078f800d001, 0xfeffffdfffefffff, 0xfffffffffffffffe}),
			EV_MSC: makeBigInt([]uint64{0x10}),
			EV_LED: makeBigInt([]uint64{0x7}),
		}); err != nil {
		return nil, err
	}

	// Sleep briefly to give Chrome and other processes time to see the new device.
	// TODO(derat): Add some way to skip this delay; it's probably unnecessary if
	// the device is created before calling chrome.New.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Using virtual keyboard device ", kw.dev)

	if kw.rw, err = Device(ctx, kw.dev); err != nil {
		kw.Close()
		return nil, err
	}

	return kw, nil
}

// Close closes the keyboard device.
func (kw *KeyboardEventWriter) Close() error {
	var firstErr error
	if kw.rw != nil {
		firstErr = kw.rw.Close()
	}
	if kw.virt != nil {
		if err := kw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Device returns the path of the underlying device, e.g. "/dev/input/event3".
// This can be useful if the keyboard also needs to be monitored by another process, e.g. evtest.
func (kw *KeyboardEventWriter) Device() string { return kw.dev }

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

	firstErr := ctx.Err()

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
	firstErr := ctx.Err()
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

	if err := testing.Sleep(ctx, 50*time.Millisecond); err != nil {
		*firstErr = errors.Wrap(err, "timeout while typing")
	}
}
