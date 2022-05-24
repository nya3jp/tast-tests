// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseAndFilterGetDiskInfo(t *testing.T) {
	const out = `
{
  "blockdevices": [
    {"name": "loop0", "type": "loop", "hotplug": false, "size": 536870912000, "state": null},
    {"name": "sda", "type": "disk", "hotplug": false, "size": 1073741824000, "state": "running"},
    {"name": "mmcblk0", "type": "disk", "hotplug": true, "size": 31268536320, "state": null},
    {"name": "mmcblk0boot0", "type": "disk", "hotplug": true, "size": 4194304, "state": null},
    {"name": "mmcblk0boot1", "type": "disk", "hotplug": true, "size": 4194304, "state": null},
    {"name":"nvme0n1", "type":"disk", "hotplug": false, "size":137438953472, "state":"live"},
    {"name":"nvme0n2", "type":"disk", "hotplug": false, "size":17179869184, "state":"live"},
    {"name":"nvme1n1", "type":"disk", "hotplug": false, "size":34359738368, "state":"live"}
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
			Hotplug: false,
			Size:    1073741824000,
			State:   "running",
		},
		{
			Name:    "mmcblk0",
			Type:    "disk",
			Hotplug: true,
			Size:    31268536320,
			State:   "",
		},
		{
			Name:    "nvme0n1",
			Type:    "disk",
			Hotplug: false,
			Size:    137438953472,
			State:   "live",
		},
		{
			Name:    "nvme0n2",
			Type:    "disk",
			Hotplug: false,
			Size:    17179869184,
			State:   "live",
		},
		{
			Name:    "nvme1n1",
			Type:    "disk",
			Hotplug: false,
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
		{"name": "sda", "type": "disk", "hotplug": false, "size": 10000, "state": "running"}
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
		{"name": "loop0", "type": "loop", "hotplug": false, "size": 536870912000, "state": null},
		{"name": "mmcblk0d1", "type": "disk", "hotplug": true, "size": 536870912000, "state": null},
		{"name": "mmcblk0", "type": "disk", "hotplug": true, "size": 10000, "state": null},
		{"name": "zram0", "type": "loop", "hotplug": false, "size": 536870912000, "state": null}
	]
}`

	exp := &Blockdevice{
		Name:    "mmcblk0",
		Type:    "disk",
		Hotplug: true,
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

	slc, err := info.SlcDevice()
	if slc != nil {
		t.Errorf("SlcDevice() returned unexpected device %+v", slc)
	}
}

func TestMainAndSlcDeviceOnly(t *testing.T) {
	const out = `
{
	"blockdevices": [
    {"name": "loop0", "type": "loop", "hotplug": false, "size": 536870912000, "state": null},
    {"name":"nvme0n1", "type":"disk", "hotplug": false, "size": 17179869184, "state":"live"},
    {"name":"nvme0n2", "type":"disk", "hotplug": false, "size": 137438953472, "state":"live"},
    {"name": "zram0", "type": "loop", "hotplug": false, "size": 536870912000, "state": null}
	]
}`

	mainDeviceExp := &Blockdevice{
		Name:    "nvme0n2",
		Type:    "disk",
		Hotplug: false,
		Size:    137438953472,
		State:   "live",
	}

	slcDeviceExp := &Blockdevice{
		Name:    "nvme0n1",
		Type:    "disk",
		Hotplug: false,
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

	slc, err := info.SlcDevice()
	if slc == nil {
		t.Fatal("SlcDevice() didn't return a valid slc device: ", err)
	}

	if !cmp.Equal(slc, slcDeviceExp) {
		t.Errorf("SlcDevice() = %+v; want %+v", slc, slcDeviceExp)
	}
}
