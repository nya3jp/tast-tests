// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseAndFilterGetDiskInfo(t *testing.T) {
	const out = `
{
  "blockdevices": [
    {"name": "loop0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null},
    {"name": "sda", "type": "disk", "hotplug": "0", "size": "1073741824000", "state": "running"},
    {"name": "mmcblk0", "type": "disk", "hotplug": "1", "size": "31268536320", "state": null},
    {"name": "mmcblk0boot0", "type": "disk", "hotplug": "1", "size": "4194304", "state": null},
    {"name": "mmcblk0boot1", "type": "disk", "hotplug": "1", "size": "4194304", "state": null},
    {"name":"nvme0n1", "type":"disk", "hotplug": "0", "size":"137438953472", "state":"live"},
    {"name":"nvme0n2", "type":"disk", "hotplug": "0", "size":"17179869184", "state":"live"},
    {"name":"nvme1n1", "type":"disk", "hotplug": "0", "size":"34359738368", "state":"live"}
  ]
}`

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}

	info = removeDisallowedDevices(info)

	exp := []*Blockdevice{
		{
			Name:    "sda",
			Type:    "disk",
			Hotplug: "0",
			Size:    1073741824000,
			State:   "running",
		},
		{
			Name:    "mmcblk0",
			Type:    "disk",
			Hotplug: "1",
			Size:    31268536320,
			State:   "",
		},
		{
			Name:    "nvme0n1",
			Type:    "disk",
			Hotplug: "0",
			Size:    137438953472,
			State:   "live",
		},
		{
			Name:    "nvme0n2",
			Type:    "disk",
			Hotplug: "0",
			Size:    17179869184,
			State:   "live",
		},
		{
			Name:    "nvme1n1",
			Type:    "disk",
			Hotplug: "0",
			Size:    34359738368,
			State:   "live",
		},
	}

	if len(info.Blockdevices) != 5 {
		t.Errorf("removeDisallowedDevices() returned %+v elements", len(info.Blockdevices))
	}

	if !cmp.Equal(info.Blockdevices, exp) {
		t.Errorf("parseDiskInfo() = %+v; want %+v", info.Blockdevices, exp)
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
		t.Fatal("CheckMainDeviceSize() returned error for a valid min size: ", err)
	}
	if err := info.CheckMainDeviceSize(15000); err == nil {
		t.Fatal("CheckMainDeviceSize() didn't return error for an invalid min size: ", err)
	}
}

func TestMainDeviceOnly(t *testing.T) {
	const out = `
{
	"blockdevices": [
		{"name": "loop0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null},
		{"name": "mmcblk0d1", "type": "disk", "hotplug": "1", "size": "536870912000", "state": null},
		{"name": "mmcblk0", "type": "disk", "hotplug": "1", "size": "10000", "state": null},
		{"name": "zram0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null}
	]
}`

	exp := &Blockdevice{
		Name:    "mmcblk0",
		Type:    "disk",
		Hotplug: "1",
		Size:    10000,
		State:   "",
	}

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}
	info = removeDisallowedDevices(info)

	dev, err := info.MainDevice()
	if err != nil {
		t.Fatal("MainDevice() didn't return a valid main device: ", err)
	}

	if !cmp.Equal(dev, exp) {
		t.Errorf("MainDevice() = %+v; want %+v", dev, exp)
	}

	secondary, err := info.SecondaryDevice()
	if secondary != nil {
		t.Errorf("SecondaryDevice() returned unexpected device %+v", secondary)
	}
}

func TestMainAndSecondaryDeviceOnly(t *testing.T) {
	const out = `
{
	"blockdevices": [
    {"name": "loop0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null},
    {"name":"nvme0n1", "type":"disk", "hotplug": "0", "size":"17179869184", "state":"live"},
    {"name":"nvme0n2", "type":"disk", "hotplug": "0", "size":"137438953472", "state":"live"},
    {"name": "zram0", "type": "loop", "hotplug": "0", "size": "536870912000", "state": null}
	]
}`

	mainDeviceExp := &Blockdevice{
		Name:    "nvme0n2",
		Type:    "disk",
		Hotplug: "0",
		Size:    137438953472,
		State:   "live",
	}

	secondaryDeviceExp := &Blockdevice{
		Name:    "nvme0n1",
		Type:    "disk",
		Hotplug: "0",
		Size:    17179869184,
		State:   "live",
	}

	info, err := parseDiskInfo([]byte(out))
	if err != nil {
		t.Fatal("parseDiskInfo() failed: ", err)
	}
	info = removeDisallowedDevices(info)

	main, err := info.MainDevice()
	if err != nil {
		t.Fatal("MainDevice() didn't return a valid main device: ", err)
	}

	if !cmp.Equal(main, mainDeviceExp) {
		t.Errorf("MainDevice() = %+v; want %+v", main, mainDeviceExp)
	}

	secondary, err := info.SecondaryDevice()
	if secondary == nil {
		t.Fatal("SecondaryDevice() didn't return a valid secondary device: ", err)
	}

	if !cmp.Equal(secondary, secondaryDeviceExp) {
		t.Errorf("SecondaryDevice() = %+v; want %+v", secondary, secondaryDeviceExp)
	}
}
