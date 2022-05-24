// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"chromiumos/tast/testutil"
)

func TestReadDevices(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// This data will be parsed incorrectly if readDevices is called in a 32-bit userspace,
	// but we currently only support running unit tests in a 64-bit userspace: https://crbug.com/918213
	if err := testutil.WriteFiles(td, map[string]string{
		procDevices: `
I: Bus=0019 Vendor=0000 Product=0005 Version=0000
N: Name="Lid Switch"
P: Phys=PNP0C0D/button/input0
S: Sysfs=/devices/LNXSYSTM:00/LNXSYBUS:00/PNP0C0D:00/input/input0
U: Uniq=
H: Handlers=event0
B: PROP=0
B: EV=21
B: SW=1

I: Bus=0011 Vendor=0001 Product=0001 Version=ab83
N: Name="AT Translated Set 2 keyboard"
P: Phys=isa0060/serio0/input0
S: Sysfs=/devices/platform/i8042/serio0/input/input3
U: Uniq=
H: Handlers=sysrq event2
B: PROP=0
B: EV=120013
B: KEY=402000000 3803078f800d001 feffffdfffefffff fffffffffffffffe
B: MSC=10
B: LED=7

I: Bus=0018 Vendor=0000 Product=0000 Version=0000
N: Name="Atmel maXTouch Touchscreen"
P: Phys=i2c-6-004b/input0
S: Sysfs=/devices/pci0000:00/0000:00:15.0/i2c_designware.0/i2c-6/i2c-ATML0001:00/input/input7
U: Uniq=
H: Handlers=event7
B: PROP=0
B: EV=b
B: KEY=400 0 0 0 0 0
B: ABS=e61800001000003`,
		// Create sysfs dirs and devices expected by getDevicePath.
		filepath.Join(sysfsDir, "devices/LNXSYSTM:00/LNXSYBUS:00/PNP0C0D:00/input/input0/event0/name"):                             "",
		filepath.Join(sysfsDir, "devices/platform/i8042/serio0/input/input3/event2/name"):                                          "",
		filepath.Join(sysfsDir, "devices/pci0000:00/0000:00:15.0/i2c_designware.0/i2c-6/i2c-ATML0001:00/input/input7/event7/name"): "",
		filepath.Join(deviceDir, "event0"): "",
		filepath.Join(deviceDir, "event2"): "",
		filepath.Join(deviceDir, "event7"): "",
	}); err != nil {
		t.Fatal(err)
	}

	infos, err := readDevices(td)
	if err != nil {
		t.Fatalf("readDevices(%q) failed: %v", td, err)
	}

	expectations := []struct {
		*devInfo
		bits  map[string]string // map from group name, e.g. "KEY", to hex bitfield
		kb    bool              // should isKeyboard return true?
		touch bool              // should isTouchscreen return true?
	}{
		{
			devInfo: &devInfo{
				name:  "Lid Switch",
				path:  filepath.Join(deviceDir, "event0"),
				phys:  "PNP0C0D/button/input0",
				devID: devID{0x19, 0x0, 0x5, 0x0},
			},
			bits: map[string]string{
				"EV": "21",
				"SW": "1",
			},
			kb:    false,
			touch: false,
		},
		{
			devInfo: &devInfo{
				name:  "AT Translated Set 2 keyboard",
				path:  filepath.Join(deviceDir, "event2"),
				phys:  "isa0060/serio0/input0",
				devID: devID{0x11, 0x1, 0x1, 0xab83},
			},
			bits: map[string]string{
				"EV":  "120013",
				"KEY": "40200000003803078f800d001feffffdfffeffffffffffffffffffffe",
				"MSC": "10",
				"LED": "7",
			},
			kb:    true,
			touch: false,
		},
		{
			devInfo: &devInfo{
				name:  "Atmel maXTouch Touchscreen",
				path:  filepath.Join(deviceDir, "event7"),
				phys:  "i2c-6-004b/input0",
				devID: devID{0x18, 0x0, 0x0, 0x0},
			},
			bits: map[string]string{
				"EV":  "b",
				"KEY": "40000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"ABS": "e61800001000003",
			},
			kb:    false,
			touch: true,
		},
	}

	if len(infos) != len(expectations) {
		t.Fatalf("readDevices(%q) = %+v; wanted %d devices", td, infos, len(expectations))
	}
	for i, exp := range expectations {
		info := infos[i]

		for pre, es := range exp.bits {
			bits, ok := info.bits[pre]
			if !ok {
				t.Errorf("device %d lacks %q bitfield", i, pre)
			} else {
				if s := bits.Text(16); s != es {
					t.Errorf("device %d has %q bits 0x%s; want 0x%s", i, pre, s, es)
				}
			}
		}
		if kb := info.isKeyboard(); kb != exp.kb {
			t.Errorf("device %d isKeyboard() = %v; want %v", i, kb, exp.kb)
		}
		if touch := info.isTouchscreen(); touch != exp.touch {
			t.Errorf("device %d isTouchscreen() = %v; want %v", i, touch, exp.touch)
		}

		info.bits = nil // can't compare *big.Int
		if !reflect.DeepEqual(*info, *exp.devInfo) {
			t.Errorf("device %d is %+v; want %+v", i, *info, *exp.devInfo)
			continue
		}
	}
}

func TestReadDevicesParseError(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	if err := testutil.WriteFiles(td, map[string]string{
		procDevices: `
I: Bus=0019 Vendor=0000 Product=0005 Version=0000
N: Name="Lid Switch"
S: Sysfs=/devices/bogus`, // missing path
	}); err != nil {
		t.Fatal(err)
	}

	// readDevices should return the error that was encountered when trying to parse the sysfs line.
	if _, err := readDevices(td); err == nil {
		t.Fatalf("readDevices(%q) didn't report expected error", td)
	}
}
