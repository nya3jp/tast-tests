// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import "testing"

func TestFlashProtectEctoolUnmarshaler(t *testing.T) {
	// It should not matter that there are tabs to the left of the examples out.
	var out = []byte(`
	Flash protect flags: 0x0000000b wp_gpio_asserted ro_at_boot ro_now
	Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
	Writable flags:      0x00000004 all_now
	`)
	var expect = FlashProtect{
		Active: FlashProtectGpioAsserted | FlashProtectRoAtBoot |
			FlashProtectRoNow,
		Valid: FlashProtectGpioAsserted | FlashProtectRoAtBoot |
			FlashProtectRoNow | FlashProtectAllNow | FlashProtectErrorStuck |
			FlashProtectErrorInconsistent,
		Writeable: FlashProtectAllNow,
	}

	var actual FlashProtect
	if err := actual.UnmarshalerEctool(out); err != nil {
		t.Fatal("Failed to unmarshal flash protect info: ", err)
	}

	if actual != expect {
		t.Fatalf("Unmarshaled flash protect block %+v doesn't match expected block %+v.", actual, expect)
	}
}

func TestFlashProtectIsSet(t *testing.T) {
	var flags = FlashProtectGpioAsserted | FlashProtectRoAtBoot | FlashProtectRoNow

	if !flags.IsSet(FlashProtectGpioAsserted) {
		t.Fatal("Flag FlashProtectGpioAsserted was reported as not set.")
	}

	if !flags.IsSet(FlashProtectRoAtBoot) {
		t.Fatal("Flag FlashProtectRoAtBoot was reported as not set.")
	}

	if !flags.IsSet(FlashProtectRoNow) {
		t.Fatal("Flag FlashProtectRoNow was reported as not set.")
	}

	if !flags.IsSet(flags) {
		t.Fatal("All flags were reported as not set.")
	}

	if flags.IsSet(flags | FlashProtectErrorInconsistent) {
		t.Fatal("Flag FlashProtectErrorInconsistent was reported as set.")
	}
}
