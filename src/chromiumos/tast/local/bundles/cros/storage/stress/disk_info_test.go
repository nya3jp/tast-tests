// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"reflect"
	"testing"
)

func TestParseGetDiskInfo(t *testing.T) {
	const out = `
{
	"blockdevices": [
		{"name": "sda", "type": "disk", "hotplug": "0", "size": "1073741824000", "state": "running"}
	]
}`

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}

	exp := Blockdevice{
		Name:    "sda",
		Type:    "disk",
		Hotplug: "0",
		Size:    1073741824000,
		State:   "running",
	}

	if !reflect.DeepEqual(info.Blockdevices[0], exp) {
		t.Errorf("parseDiskInfo() = %+v; want %+v", info.Blockdevices[0], exp)
	}
}

func TestParseAndFilterGetDiskInfo(t *testing.T) {
	const out = `
	{
		"blockdevices": [
			{"name": "loop0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null},
			{"name": "sda", "type": "disk", "hotplug": "0", "size": "1073741824000", "state": "running"}
		]
	}`

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}

	info = filterNonDiskDevices(info)

	exp := Blockdevice{
		Name:    "sda",
		Type:    "disk",
		Hotplug: "0",
		Size:    1073741824000,
		State:   "running",
	}

	if len(info.Blockdevices) != 1 {
		t.Errorf("filterNonDiskDevices() returned %+v elements", len(info.Blockdevices))
	}

	if !reflect.DeepEqual(info.Blockdevices[0], exp) {
		t.Errorf("parseDiskInfo() = %+v; want %+v", info.Blockdevices[0], exp)
	}
}

func TestGetMainDeviceSize(t *testing.T) {
	const out = `
{
	"blockdevices": [
		{"name": "sda", "type": "disk", "hotplug": "0", "size": "10000", "state": "running"}
	]
}`

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}

	if err := info.CheckMainDeviceSize(5000); err != nil {
		t.Error("CheckMainDeviceSize() returned error for a valid min size")
	}
	if err := info.CheckMainDeviceSize(15000); err == nil {
		t.Error("CheckMainDeviceSize() didn't return error for an invalid min size")
	}
}

func TestGetMainDevice(t *testing.T) {
	const out = `
{
	"blockdevices": [
		{"name": "loop0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null},
		{"name": "mmcblk0d1", "type": "disk", "hotplug": "0", "size": "10000", "state": "running"},
		{"name": "zram0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null}
	]
}`

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}

	dev, err := info.GetMainDevice()
	if err != nil {
		t.Error("GetMainDevice() didn't return a valid main device")
	}

	if dev.Name != "mmcblk0d1" {
		t.Errorf("GetMainDevice() returned wrong device %+v", dev)
	}
}
