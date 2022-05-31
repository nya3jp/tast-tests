// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// KeyboardEventWriter supports injecting events into a keyboard device.
type KeyboardEventWriter struct {
	rw               *RawEventWriter
	virt             *os.File         // if non-nil, used to hold a virtual device open
	fast             bool             // if true, do not sleep after type; useful for unit tests
	dev              string           // path to underlying device in /dev/input
	topRowLayoutType TopRowLayoutType // layout type of the top row of the keyboard
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
		foundKB, infoPath, err := FindPhysicalKeyboard(ctx)
		if err != nil {
			return nil, err
		}
		if foundKB {
			rw, err := Device(ctx, infoPath)
			if err != nil {
				return nil, err
			}
			return &KeyboardEventWriter{rw: rw, dev: infoPath}, nil
		}
	}

	// If we didn't find a real keyboard, create a virtual one.
	return VirtualKeyboard(ctx)
}

// FindPhysicalKeyboard iterates over devices and returns path for physical keyboard
// otherwise returns boolean stating a physical keyboard was not found
func FindPhysicalKeyboard(ctx context.Context) (bool, string, error) {
	infos, err := readDevices("")
	if err != nil {
		return false, "", errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		if info.isKeyboard() && info.phys != "" {
			testing.ContextLogf(ctx, "Using existing keyboard device %+v", info)
			return true, info.path, nil
		}
	}
	return false, "", nil
}

// FindPowerKeyDevice iterates over devices and returns path for cros_ec_buttons
// if it exists, otherwise returns the regular physical keyboard.
func FindPowerKeyDevice(ctx context.Context) (bool, string, error) {
	infos, err := readDevices("")
	if err != nil {
		return false, "", errors.Wrap(err, "failed to read devices")
	}
	// If cros_ec_buttons or Power Button is a device, use that for power key, otherwise use physical keyboard device.
	for _, info := range infos {
		if strings.Contains(info.name, "cros_ec_buttons") || strings.Contains(info.name, "Power Button") {
			testing.ContextLogf(ctx, "Using %s device %+v", info.name, info)
			return true, info.path, nil
		}
	}
	return FindPhysicalKeyboard(ctx)
}

// virtualKeyboard creates a virtual keyboard device and returns an EventWriter that injects events into it.
func virtualKeyboard(ctx context.Context, busType uint16) (*KeyboardEventWriter, error) {
	kw := &KeyboardEventWriter{}

	// Include our PID in the device name to be extra careful in case an old bundle process hasn't exited.
	name := fmt.Sprintf("Tast virtual keyboard %d.%d", os.Getpid(), nextVirtKbdNum)
	nextVirtKbdNum++
	testing.ContextLogf(ctx, "Creating virtual keyboard device %q", name)

	// These values are copied from the "AT Translated Set 2 keyboard" device on an amd64-generic VM.
	var err error
	if kw.dev, kw.virt, err = createVirtual(name, devID{busType, 0x1, 0x1, 0xab41}, 0, 0x120013,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x402000000, 0x3803078f800d001, 0xfeffffdfffefffff, 0xfffffffffffffffe}),
			EV_MSC: big.NewInt(1 << MSC_SCAN),
			EV_LED: big.NewInt(1<<LED_NUML | 1<<LED_CAPSL | 1<<LED_SCROLLL),
		}, nil); err != nil {
		return nil, err
	}

	// Sleep briefly to give Chrome and other processes time to see the new device.
	// This delay is probably unnecessary if the device is created before calling chrome.New,
	// but that's not guaranteed to happen.
	// TODO(crbug.com/1015264): Remove the hard-coded sleep.
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

// VirtualKeyboard creates a virtual keyboard device and returns an EventWriter that injects events into it.
func VirtualKeyboard(ctx context.Context) (*KeyboardEventWriter, error) {
	// Hardcode as BUS_USB (0x3), as BUS_I8042 (0x11) doesn't work on some hardware.
	// See https://crrev.com/c/1407138 for more discussion.
	return virtualKeyboard(ctx, 0x3) // BUS_USB = 0x3 as an usb keyboard from input.h
}

// VirtualKeyboardWithBusType creates a virtual keyboard device with specific bus type and returns an EventWriter that injects events into it.
func VirtualKeyboardWithBusType(ctx context.Context, busType uint16) (*KeyboardEventWriter, error) {
	return virtualKeyboard(ctx, busType)
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
// If kw represents a keyboard with a custom top row, we will also send a EV_MSC
//	event mapped from topRowScanCodeMap
// If firstErr points at a non-nil error, no events are written.
// If an error is encountered, it is saved to the address pointed to by firstErr.
func (kw *KeyboardEventWriter) sendKey(ec EventCode, val int32, firstErr *error) {
	if *firstErr == nil {
		// If top row is a custom layout, EV_MSC event must also be written for system keys
		// (eg. Brightness) to be processed correctly.
		if kw.topRowLayoutType == LayoutCustom {
			// Find correct scan code based on the event code
			sc, prs := topRowScanCodeMap[ec]
			if prs {
				*firstErr = kw.rw.Event(EV_MSC, MSC_SCAN, sc)
			}
		}
	}
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
		kw.sleepAfterType(ctx, &firstErr)
		kw.sendKey(k.code, 0, &firstErr)

		if shifted && (i+1 == len(keys) || !keys[i+1].shifted) {
			kw.sendKey(KEY_LEFTSHIFT, 0, &firstErr)
			shifted = false
		}

		kw.sleepAfterType(ctx, &firstErr)
	}

	return firstErr
}

type keyboardEventType uint8

const (
	keyPress keyboardEventType = 1 << iota
	keyRelease
)

func (kw *KeyboardEventWriter) accel(ctx context.Context, s string, eventType keyboardEventType) error {
	keys, err := parseAccel(s)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", s)
	}
	if len(keys) == 0 {
		return errors.Errorf("no keys found in %q", s)
	}

	// Press the keys in forward order and then release them in reverse order.
	firstErr := ctx.Err()
	if eventType&keyPress != 0 {
		for i := 0; i < len(keys); i++ {
			kw.sendKey(keys[i], 1, &firstErr)
			kw.sleepAfterType(ctx, &firstErr)
		}
	}
	if eventType&keyRelease != 0 {
		for i := len(keys) - 1; i >= 0; i-- {
			kw.sendKey(keys[i], 0, &firstErr)
			kw.sleepAfterType(ctx, &firstErr)
		}
	}
	return firstErr
}

// Accel injects a sequence of key events simulating the accelerator (a.k.a. hotkey) described by s being typed.
// Accelerators are described as a sequence of '+'-separated, case-insensitive key characters or names.
// In addition to non-whitespace characters that are present on a QWERTY keyboard, the following key names may be used:
//
//	Modifiers:     "Ctrl", "Alt", "Search", "Shift"
//	Whitespace:    "Enter", "Space", "Tab", "Backspace"
//	Function keys: "F1", "F2", ..., "F12"
//
// "Shift" must be included for keys that are typed using Shift; for example, use "Ctrl+Shift+/" rather than "Ctrl+?".
func (kw *KeyboardEventWriter) Accel(ctx context.Context, s string) error {
	return kw.accel(ctx, s, keyPress|keyRelease)
}

// AccelPress injects a sequence of key events simulating pressing the accelerator (a.k.a. hotkey) described by s.
func (kw *KeyboardEventWriter) AccelPress(ctx context.Context, s string) error {
	return kw.accel(ctx, s, keyPress)
}

// AccelRelease injects a sequence of key events simulating release the accelerator (a.k.a. hotkey) described by s.
func (kw *KeyboardEventWriter) AccelRelease(ctx context.Context, s string) error {
	return kw.accel(ctx, s, keyRelease)
}

// sleepAfterType sleeps for short time. It is supposed to be called after key strokes.
// Without sleeping between keystrokes, the omnibox seems to produce scrambled text.
// Presumably there's a bug in Chrome's input stack or the omnibox code.
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

// TypeKey injects a pair of a keypress event and a keyrelease keyevent.
// It can be used to inject non-character key events.
func (kw *KeyboardEventWriter) TypeKey(ctx context.Context, ec EventCode) error {
	firstErr := ctx.Err()
	kw.sendKey(ec, 1, &firstErr)
	kw.sleepAfterType(ctx, &firstErr)
	kw.sendKey(ec, 0, &firstErr)
	kw.sleepAfterType(ctx, &firstErr)
	return firstErr
}

// TypeSequence injects key events suitable given a string slice seq, where seq
// is a combination of rune keys and accelerators.
// For each string s, it uses Type() to inject a key event if the len(s) = 1,
// and uses Accel() to inject a key event if the len(s) > 1.
// E.g., when calling TypeSequence({"S","e","q","space"}), it calls
// Type("S"), Type("e"), Type("q") and Accel("space") respectively.
func (kw *KeyboardEventWriter) TypeSequence(ctx context.Context, seq []string) error {
	for _, s := range seq {
		switch len(s) {
		case 0:
			return errors.New("no key event for empty string")
		case 1:
			if err := kw.Type(ctx, s); err != nil {
				return errors.Errorf("failed to type %q", s)
			}
		default:
			if err := kw.Accel(ctx, s); err != nil {
				return errors.Errorf("failed to type %q", s)
			}
		}
	}
	return nil
}

// Below are the action versions of keyboard methods.
// They enable easy chaining of typing with the ui library.

// TypeAction returns a function that types the specified string.
func (kw *KeyboardEventWriter) TypeAction(s string) action.Action {
	return func(ctx context.Context) error {
		return kw.Type(ctx, s)
	}
}

// TypeKeyAction returns a function that types the specified key.
func (kw *KeyboardEventWriter) TypeKeyAction(ec EventCode) action.Action {
	return func(ctx context.Context) error {
		return kw.TypeKey(ctx, ec)
	}
}

// AccelAction returns a function injects a sequence of key events simulating the accelerator (a.k.a. hotkey) described by s being typed.
func (kw *KeyboardEventWriter) AccelAction(s string) action.Action {
	return func(ctx context.Context) error {
		return kw.Accel(ctx, s)
	}
}

// AccelPressAction returns a function injects a sequence of key events simulating pressing the accelerator (a.k.a. hotkey) described by s.
func (kw *KeyboardEventWriter) AccelPressAction(s string) action.Action {
	return func(ctx context.Context) error {
		return kw.AccelPress(ctx, s)
	}
}

// AccelReleaseAction returns a function injects a sequence of key events simulating release the accelerator (a.k.a. hotkey) described by s.
func (kw *KeyboardEventWriter) AccelReleaseAction(s string) action.Action {
	return func(ctx context.Context) error {
		return kw.AccelRelease(ctx, s)
	}
}

// TypeSequenceAction returns an action wrapper for TypeSequence().
func (kw *KeyboardEventWriter) TypeSequenceAction(seq []string) action.Action {
	return func(ctx context.Context) error {
		return kw.TypeSequence(ctx, seq)
	}
}
