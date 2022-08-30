// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import "testing"

func TestResetFlagsEctoolUnmarshaler(t *testing.T) {
	var out = "0x00000c02\n"
	var expect = ResetFlags(ResetFlagResetPin | ResetFlagSysjump | ResetFlagHard)

	flags, err := unmarshalEctoolResetFlags(out)
	if err != nil {
		t.Fatal("Failed to unmarshal reset flags: ", err)
	}
	actual := ResetFlags(flags)

	if actual != expect {
		t.Fatalf("Unmarshaled reset flags  %+v doesn't match expected flags %+v.", actual, expect)
	}
}

func TestResetFlagsIsSet(t *testing.T) {
	var flags ResetFlags

	if flags.IsSet(ResetFlagPowerOn) {
		t.Fatal("Flag ResetFlagsPowerOn was reported as set.")
	}

	flags = ResetFlagPowerOn
	if !flags.IsSet(ResetFlagPowerOn) {
		t.Fatal("Flag ResetFlagsPowerOn was reported as not set.")
	}
}
