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

	const fn = "devices"
	if err := testutil.WriteFiles(td, map[string]string{
		fn: `
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
H: Handlers=sysrq event3
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
B: ABS=e61800001000003
`}); err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(td, fn)
	infos, err := readDevices(p)
	if err != nil {
		t.Fatalf("readDevices(%q) failed: %v", p, err)
	}

	expectations := []struct {
		*devInfo
		bits map[string]string // map from group name, e.g. "KEY", to hex bitfield
		kb   bool              // should isKeyboard return true?
	}{
		{
			devInfo: &devInfo{
				name:    "Lid Switch",
				path:    filepath.Join(deviceDir, "event0"),
				bus:     0x19,
				vendor:  0x0,
				product: 0x5,
				version: 0x0,
			},
			bits: map[string]string{
				"EV": "21",
				"SW": "1",
			},
			kb: false,
		},
		{
			devInfo: &devInfo{
				name:    "AT Translated Set 2 keyboard",
				path:    filepath.Join(deviceDir, "event3"),
				bus:     0x11,
				vendor:  0x1,
				product: 0x1,
				version: 0xab83,
			},
			bits: map[string]string{
				"EV":  "120013",
				"KEY": "40200000003803078f800d001feffffdfffeffffffffffffffffffffe",
				"MSC": "10",
				"LED": "7",
			},
			kb: true,
		},
		{
			devInfo: &devInfo{
				name:    "Atmel maXTouch Touchscreen",
				path:    filepath.Join(deviceDir, "event7"),
				bus:     0x18,
				vendor:  0x0,
				product: 0x0,
				version: 0x0,
			},
			bits: map[string]string{
				"EV":  "b",
				"KEY": "40000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"ABS": "e61800001000003",
			},
			kb: false,
		},
	}

	if len(infos) != len(expectations) {
		t.Fatalf("readDevices(%q) = %+v; wanted %d devices", p, infos, len(expectations))
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
		info.bits = nil // can't compare *big.Int
		if !reflect.DeepEqual(*info, *exp.devInfo) {
			t.Errorf("device %d is %+v; want %+v", i, *info, *exp.devInfo)
			continue
		}
	}
}
