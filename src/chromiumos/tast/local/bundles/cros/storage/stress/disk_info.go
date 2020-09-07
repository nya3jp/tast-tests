// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Blockdevice represents information about a single storage device as reported by lsblk.
type Blockdevice struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Hotplug string `json:"hotplug"`
	Size    int64  `json:"size,string"`
	State   string `json:"state"`
}

// DiskInfo is a serializable structure representing output of lsblk command.
type DiskInfo struct {
	Blockdevices []*Blockdevice `json:"blockdevices"`
}

// MainDevice returns the main storage device from a list of available devices.
// The method returns the device with the biggest size if multiple present.
func (d DiskInfo) MainDevice() (*Blockdevice, error) {
	var bestMatch *Blockdevice
	for _, device := range d.Blockdevices {
		if bestMatch == nil || bestMatch.Size < device.Size {
			bestMatch = device
		}
	}
	if bestMatch != nil {
		return bestMatch, nil
	}
	return nil, errors.Errorf("unable to identify main storage device from devices: %+v", d)
}

// SecondaryDevice returns the secondary storage device from a list of available
// devices. The method returns the device with the smallest size.
func (d DiskInfo) SecondaryDevice() (*Blockdevice, error) {
	if d.DeviceCount() < 2 {
		return nil, errors.Errorf("no secondary devices present: %+v", d)
	}
	var bestMatch *Blockdevice
	for _, device := range d.Blockdevices {
		if bestMatch == nil || bestMatch.Size > device.Size {
			bestMatch = device
		}
	}
	return bestMatch, nil
}

// DeviceCount returns number of found valid block devices on the system.
func (d DiskInfo) DeviceCount() int {
	return len(d.Blockdevices)
}

// CheckMainDeviceSize verifies that the size of the main storage disk is more than
// the given minimal size. Otherwise, an error is returned.
func (d DiskInfo) CheckMainDeviceSize(minSize int64) error {
	device, err := d.MainDevice()
	if err != nil {
		return errors.Wrap(err, "failed getting main storage disk")
	}

	if device.Size < minSize {
		return errors.Errorf("main storage device size too small: %v", device.Size)
	}
	return nil
}

// SaveDiskInfo dumps disk info to an external file with a given file name.
// The information is saved in JSON format.
func (d DiskInfo) SaveDiskInfo(fileName string) error {
	file, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling disk info to JSON")
	}
	err = ioutil.WriteFile(fileName, file, 0644)
	if err != nil {
		return errors.Wrap(err, "failed saving disk info to file")
	}
	return nil
}

// ReadDiskInfo returns storage information as reported by lsblk tool.
// Only disk devices are returns.
func ReadDiskInfo(ctx context.Context) (*DiskInfo, error) {
	cmd := testexec.CommandContext(ctx, "lsblk", "-b", "-d", "-J", "-o", "NAME,TYPE,HOTPLUG,SIZE,STATE")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsblk")
	}
	diskInfo, err := parseDiskInfo(out)
	if err != nil {
		return nil, err
	}
	return filterNotAllowedDevices(diskInfo), nil
}

func parseDiskInfo(out []byte) (*DiskInfo, error) {
	var result DiskInfo
	// TODO(dlunev): make sure the format is the same for all kernel versions.
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse lsblk result")
	}
	return &result, nil
}

// filterNotAllowedDevices filters out devices which are not matching the regexp
// or are not disks
// TODO(dlunev): We should consider mmc devices only if they are 'root' devices
// for there is no reliable way to differentiate removable mmc.
func filterNotAllowedDevices(diskInfo *DiskInfo) *DiskInfo {
	var devices []*Blockdevice
	devRegExp := regexp.MustCompile(`(sda|nvme\dn\d|mmcblk\d)$`)
	for _, device := range diskInfo.Blockdevices {
		if device.Type == "disk" && devRegExp.MatchString(device.Name) {
			devices = append(devices, device)
		}
	}
	return &DiskInfo{Blockdevices: devices}
}
